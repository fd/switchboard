package dispatcher

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/fd/switchboard/pkg/hosts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/marpie/godhcp"
	"golang.org/x/net/context"
)

func (vnet *VNET) dispatchDHCP(ctx context.Context) chan<- *Packet {
	var in = make(chan *Packet)

	vnet.wg.Add(1)
	go func() {
		defer vnet.wg.Done()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

	LOOP:
		for {
			select {

			case pkt := <-in:
				vnet.handleDHCP(pkt)

			case <-ticker.C:
				tab := vnet.hosts.GetTable()

				gateway := tab.LookupByName("gateway")
				if gateway == nil {
					continue LOOP
				}

				controller := tab.LookupByName("controller")
				if controller == nil {
					continue
				}
				if !controller.Up {
					continue
				}
				if controller.IPv4 != nil {
					// TODO renew
					continue
				}

				vnet.requestDHCPLease(controller)

			case <-ctx.Done():
				return
			}
		}
	}()

	return in
}

func (vnet *VNET) handleDHCP(pkt *Packet) {
	defer pkt.Release()

	tab := vnet.hosts.GetTable()
	controller := tab.LookupByName("controller")
	if controller == nil {
		return
	}

	if !bytes.Equal(controller.MAC, pkt.Eth.DstMAC) {
		return
	}

	msg, err := dhcp.ReadMessage(pkt.UDP.Payload)
	if err != nil {
		log.Printf("DCHP/error: %s", err)
		return
	}

	if msg.Type != dhcp.MessageTypeReply {
		return
	}

	opt := msg.Options[dhcp.OptionCodeDHCPMessageType]
	if opt == nil || len(opt.Value) != 1 {
		return
	}

	switch opt.Value[0] {
	case dhcp.DHCPMessageTypeOffer:
		log.Printf("DHCP/OFFER")
		vnet.handleDHCPOffer(pkt, msg, controller)
	case dhcp.DHCPMessageTypeAck:
		log.Printf("DHCP/ACK")
		vnet.handleDHCPAck(pkt, msg, controller)
	}
}

func (vnet *VNET) handleDHCPOffer(pkt *Packet, offer *dhcp.Message, host *hosts.Host) {
	if offer.YourIPAddress == nil {
		return
	}

	msg := &dhcp.Message{}

	msg.ClientMAC = host.MAC

	msg.Type = dhcp.MessageTypeRequest
	msg.HardwareType = dhcp.MessageHardwareTypeEthernet
	msg.HardwareAddressLength = 6
	msg.Hops = 0
	msg.TransactionID = offer.TransactionID
	msg.Options = map[uint8]*dhcp.Option{

		dhcp.OptionCodeDHCPMessageType: {
			Value: []byte{dhcp.DHCPMessageTypeRequest},
		},

		dhcp.OptionCodeDHCPClientidentifier: {
			Value: append([]byte{1}, msg.ClientMAC[:6]...),
		},

		dhcp.OptionCodeDHCPRequestedIPAddress: {
			Value: offer.YourIPAddress.To4(),
		},

		dhcp.OptionCodeDHCPServerIdentifier: offer.Options[dhcp.OptionCodeDHCPServerIdentifier],

		dhcp.OptionCodeDHCPMaximumMessageSize: {
			Value: []byte{2, 64}, // 576
		},

		dhcp.OptionCodeDHCPParameterRequestList: {
			Value: []byte{
				dhcp.OptionCodeSubnetMask,
				dhcp.OptionCodeRouter,
				dhcp.OptionCodeDomainNameServer,
				dhcp.OptionCodeHostName,
				dhcp.OptionCodeDomainName,
				dhcp.OptionCodeBroadcastAddress,
				dhcp.OptionCodeNetworkTimeProtocolServers,
			},
		},

		dhcp.OptionCodeDHCPVendorclassidentifier: {
			Value: []byte("swtchbrd 1.23.1"),
		},

		dhcp.OptionCodeEnd: {},
	}

	buf := gopacket.NewSerializeBuffer()

	opts := gopacket.SerializeOptions{}
	opts.FixLengths = true
	opts.ComputeChecksums = true

	ipv4 := &layers.IPv4{
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
		Version:  4,
		Protocol: layers.IPProtocolUDP,
		TTL:      64,
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(68),
		DstPort: layers.UDPPort(67),
	}

	udp.SetNetworkLayerForChecksum(ipv4)

	err := gopacket.SerializeLayers(buf, opts,
		&layers.Ethernet{
			SrcMAC:       msg.ClientMAC,
			DstMAC:       layers.EthernetBroadcast,
			EthernetType: layers.EthernetTypeIPv4,
		},
		ipv4,
		udp,
		gopacket.Payload(writeDHCPMessage(msg)))
	if err != nil {
		log.Printf("DCHP/error: %s", err)
		return
	}

	// opkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.NoCopy)
	// log.Printf("WRITE: %08x %s\n", 0, opkt.Dump())

	_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
	if err != nil {
		log.Printf("DCHP/error: %s", err)
		return
	}

	return
}

func (vnet *VNET) handleDHCPAck(pkt *Packet, ack *dhcp.Message, host *hosts.Host) {
	if ack.YourIPAddress == nil {
		return
	}
	if bytes.Equal(host.IPv4, ack.YourIPAddress) {
		return
	}

	vnet.hosts.HostSetIPv4(host.ID, CloneIP(ack.YourIPAddress))
	log.Printf("DCHP leased: %s", ack.YourIPAddress)

	// sudo route -n add -net 172.18.0.0/16 192.168.128.7
	log.Printf("exec: %v", []string{"route", "-n", "add", "-net", "172.18.0.0/16", ack.YourIPAddress.String()})
	err := exec.Command("sudo", "route", "-n", "add", "-net", "172.18.0.0/16", ack.YourIPAddress.String()).Run()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}

	var (
		ifaceName   string
		coIfaceName string
	)
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}
	for _, iface := range ifaces {
		if !bytes.Equal(iface.HardwareAddr, pkt.Eth.SrcMAC) {
			continue
		}
		output, err := exec.Command("ifconfig", iface.Name).Output()
		if err != nil {
			log.Printf("ROUTE/error: %s", err)
			return
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "member:") {
				continue
			}
			line = strings.TrimPrefix(line, "member:")
			line = strings.TrimSpace(line)
			if idx := strings.IndexByte(line, ' '); idx > 0 {
				line = line[:idx]
			}
			ifaceName = iface.Name
			coIfaceName = line
			break
		}
		break
	}

	// sudo ifconfig bridge100 -hostfilter en4
	log.Printf("exec: %v", []string{"ifconfig", ifaceName, "-hostfilter", coIfaceName})
	err = exec.Command("sudo", "ifconfig", ifaceName, "-hostfilter", coIfaceName).Run()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}
}

func (vnet *VNET) requestDHCPLease(host *hosts.Host) {
	msg := &dhcp.Message{}

	msg.ClientMAC = host.MAC

	msg.Type = dhcp.MessageTypeRequest
	msg.HardwareType = dhcp.MessageHardwareTypeEthernet
	msg.HardwareAddressLength = 6
	msg.Hops = 0
	msg.TransactionID = rand.Uint32()
	msg.Options = map[uint8]*dhcp.Option{

		dhcp.OptionCodeDHCPMessageType: {
			Value: []byte{dhcp.DHCPMessageTypeDiscover},
		},

		dhcp.OptionCodeDHCPMaximumMessageSize: {
			Value: []byte{2, 64}, // 576
		},

		dhcp.OptionCodeDHCPClientidentifier: {
			Value: append([]byte{1}, msg.ClientMAC[:6]...),
		},

		dhcp.OptionCodeHostName: {
			Value: []byte(host.Name),
		},

		dhcp.OptionCodeDHCPParameterRequestList: {
			Value: []byte{
				dhcp.OptionCodeSubnetMask,
				dhcp.OptionCodeRouter,
				dhcp.OptionCodeDomainNameServer,
				dhcp.OptionCodeHostName,
				dhcp.OptionCodeDomainName,
				dhcp.OptionCodeBroadcastAddress,
				dhcp.OptionCodeNetworkTimeProtocolServers,
			},
		},

		dhcp.OptionCodeDHCPVendorclassidentifier: {
			Value: []byte("swtchbrd 1.23.1"),
		},

		dhcp.OptionCodeEnd: {},
	}

	buf := gopacket.NewSerializeBuffer()

	opts := gopacket.SerializeOptions{}
	opts.FixLengths = true
	opts.ComputeChecksums = true

	ipv4 := &layers.IPv4{
		SrcIP:    net.IPv4(0, 0, 0, 0),
		DstIP:    net.IPv4(255, 255, 255, 255),
		Version:  4,
		Protocol: layers.IPProtocolUDP,
		TTL:      64,
	}

	udp := &layers.UDP{
		SrcPort: layers.UDPPort(68),
		DstPort: layers.UDPPort(67),
	}

	udp.SetNetworkLayerForChecksum(ipv4)

	err := gopacket.SerializeLayers(buf, opts,
		&layers.Ethernet{
			SrcMAC:       msg.ClientMAC,
			DstMAC:       layers.EthernetBroadcast,
			EthernetType: layers.EthernetTypeIPv4,
		},
		ipv4,
		udp,
		gopacket.Payload(writeDHCPMessage(msg)))
	if err != nil {
		log.Printf("DCHP/error: %s", err)
		return
	}

	// opkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.NoCopy)
	// log.Printf("WRITE: %08x %s\n", 0, opkt.Dump())

	_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
	if err != nil {
		log.Printf("DCHP/error: %s", err)
		return
	}

	return
}

func writeUint32(w io.Writer, i uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], i)
	w.Write(buf[:])
}

func writeUint16(w io.Writer, i uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], i)
	w.Write(buf[:])
}

func writeDHCPMessage(msg *dhcp.Message) []byte {
	var buf bytes.Buffer
	var blank [128]byte

	buf.WriteByte(msg.Type)
	buf.WriteByte(msg.HardwareType)
	buf.WriteByte(msg.HardwareAddressLength)
	buf.WriteByte(msg.Hops)
	writeUint32(&buf, msg.TransactionID)
	writeUint16(&buf, msg.SecondsElapsed)
	writeUint16(&buf, msg.Flags)
	if msg.ClientIPAdress != nil {
		buf.Write(msg.ClientIPAdress.To4())
	} else {
		buf.Write(blank[:4])
	}
	if msg.YourIPAddress != nil {
		buf.Write(msg.YourIPAddress.To4())
	} else {
		buf.Write(blank[:4])
	}
	if msg.NextServerIPAddress != nil {
		buf.Write(msg.NextServerIPAddress.To4())
	} else {
		buf.Write(blank[:4])
	}
	if msg.RelayIPAddress != nil {
		buf.Write(msg.RelayIPAddress.To4())
	} else {
		buf.Write(blank[:4])
	}
	buf.Write(msg.ClientMAC)
	buf.Write(blank[:16-len(msg.ClientMAC)])
	buf.WriteString(msg.ServerHostName)
	buf.Write(blank[:64-len(msg.ServerHostName)])
	buf.WriteString(msg.File)
	buf.Write(blank[:128-len(msg.File)])
	writeUint32(&buf, dhcp.MagicCookie)

	for i := 0; i < 256; i++ {
		code := uint8(i)
		opt := msg.Options[code]
		if opt == nil {
			continue
		}

		buf.WriteByte(code)

		if code == dhcp.OptionCodeEnd || code == dhcp.OptionCodePad {
			break
		}

		buf.WriteByte(uint8(len(opt.Value)))
		buf.Write(opt.Value)
	}

	return buf.Bytes()
}
