package dispatcher

import (
	"bytes"
	"errors"
	"log"
	"net"
	"os/exec"

	"github.com/fd/switchboard/pkg/hosts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/context"
)

func (vnet *VNET) dispatchARP(ctx context.Context) chan<- *Packet {
	var in = make(chan *Packet)

	vnet.wg.Add(1)
	go func() {
		defer vnet.wg.Done()

		for {
			var pkt *Packet

			select {
			case pkt = <-in:
			case <-ctx.Done():
				return
			}

			switch pkt.ARP.Operation {

			case layers.ARPRequest:
				vnet.handleARPRequest(pkt)

			case layers.ARPReply:
				// vnet.handleARPReply(pkt)
				// log.Printf("ARP REP: %08x %s\n", pkt.Flags, pkt.Dump())
				pkt.Release()

			default:
				pkt.Release()

			}

		}
	}()

	return in
}

func (vnet *VNET) handleARPRequest(pkt *Packet) {
	defer pkt.Release()
	// log.Printf("ARP REQ: %08x %s\n", pkt.Flags, pkt.Dump())

	var (
		eth = pkt.Eth
		arp = pkt.ARP
	)

	if !bytes.Equal(eth.DstMAC, layers.EthernetBroadcast[:]) {
		// ignore
		return
	}

	table := vnet.hosts.GetTable()

	if table.LookupByName("gateway") == nil {
		err := vnet.insertGateway(pkt)
		if err != nil {
			log.Printf("setup error: %s", err)
		}
		table = vnet.hosts.GetTable()
	}

	if table.LookupByName("controller") == nil {
		err := vnet.insertController(pkt)
		if err != nil {
			log.Printf("setup error: %s", err)
		}
		table = vnet.hosts.GetTable()
	}

	if bytes.Equal(arp.SourceProtAddress, arp.DstProtAddress) {
		// was announce
		return
	}

	var (
		host            *hosts.Host
		hostProtAddress net.IP
	)

	if arp.ProtAddressSize == 4 {
		host = table.LookupByIPv4(net.IP(arp.DstProtAddress).To4())
	} else {
		host = table.LookupByIPv6(net.IP(arp.DstProtAddress).To16())
	}

	if host == nil {
		// ignore
		// log.Printf("ARP: DST: %s (unknown)\n", net.IP(arp.DstProtAddress))
		// table.Dump()
		return
	}
	if !host.Up {
		// ignore
		// log.Printf("ARP: DST: %s (down)\n", net.IP(arp.DstProtAddress))
		return
	}

	// log.Printf("ARP: DST: %s (up)\n", net.IP(arp.DstProtAddress))

	if arp.ProtAddressSize == 4 {
		if len(host.IPv4Addrs) == 0 {
			// ignore
			return
		}
		hostProtAddress = host.IPv4Addrs[0].To4()
	} else {
		if len(host.IPv6Addrs) == 0 {
			// ignore
			return
		}
		hostProtAddress = host.IPv6Addrs[0].To16()
	}
	if hostProtAddress == nil {
		// ignore
		// log.Printf("ARP: DST: %s (unreachable)\n", net.IP(arp.DstProtAddress))
		return
	}

	buf := gopacket.NewSerializeBuffer()

	opts := gopacket.SerializeOptions{}
	opts.FixLengths = true
	opts.ComputeChecksums = true

	err := gopacket.SerializeLayers(buf, opts,
		&layers.Ethernet{
			SrcMAC:       host.MAC,
			DstMAC:       eth.SrcMAC,
			EthernetType: eth.EthernetType,
		},
		&layers.ARP{
			AddrType:          arp.AddrType,
			Protocol:          arp.Protocol,
			Operation:         layers.ARPReply,
			SourceHwAddress:   host.MAC,
			SourceProtAddress: hostProtAddress,
			DstHwAddress:      arp.SourceHwAddress,
			DstProtAddress:    arp.SourceProtAddress,
		})
	if err != nil {
		log.Printf("error[ARP]: %s", err)
		return
	}

	// opkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.NoCopy)
	// log.Printf("WRITE: %08x %s\n", 0, opkt.Dump())

	_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
	if err != nil {
		log.Printf("error[ARP]: %s", err)
		return
	}

	// vnet.announceARP(host)
}

func (vnet *VNET) insertGateway(pkt *Packet) error {
	if pkt.ARP.ProtAddressSize != 4 {
		// ignore
		return nil
	}

	host, err := vnet.hosts.AddHost(&hosts.Host{
		ID:    "d9c62f0c-7936-4384-8d85-4587561a7142",
		Name:  "gateway",
		Local: true,

		MAC:       CloneHwAddress(pkt.ARP.SourceHwAddress),
		IPv4Addrs: []net.IP{CloneIP(pkt.ARP.SourceProtAddress)},
		IPv6Addrs: []net.IP{net.ParseIP("fd4c:bd56:5cee:8000::1")},

		Up: true,
	})
	if err != nil {
		return err
	}
	log.Printf("insert gateway: %v", host)

	{
		var (
			iface net.Interface
			found bool
		)

		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		for _, i := range ifaces {
			if bytes.Equal(i.HardwareAddr, pkt.ARP.SourceHwAddress) {
				// found iface
				iface = i
				found = true
				break
			}
		}
		if !found {
			return errors.New("unable to find interface")
		}

		err = exec.Command("sudo", "ifconfig", iface.Name, "inet6", "fd4c:bd56:5cee:8000::1", "prefixlen", "48").Run()
		if err != nil {
			return err
		}
	}

	return nil
}

func (vnet *VNET) insertController(pkt *Packet) error {
	if pkt.ARP.ProtAddressSize != 4 {
		// ignore
		return nil
	}

	host, err := vnet.hosts.AddHost(&hosts.Host{
		ID:    "7ce86376-34f0-4951-bead-6152c8291f1c",
		Name:  "controller",
		Local: true,

		MAC:       CloneHwAddress(vnet.iface.HardwareAddr()),
		IPv6Addrs: []net.IP{net.ParseIP("fd4c:bd56:5cee:8000::2")},

		Up: true,
	})
	if err != nil {
		return err
	}
	log.Printf("insert controller: %v", host)

	// vnet.announceARP(host)
	return nil
}

// func (vnet *VNET) announceARP(host *hosts.Host) {
// 	if host == nil {
// 		return
// 	}
// 	if !host.Up {
// 		return
// 	}
// 	if host.IPv4 == nil {
// 		return
// 	}
//
// 	buf := gopacket.NewSerializeBuffer()
//
// 	opts := gopacket.SerializeOptions{}
// 	opts.FixLengths = true
// 	opts.ComputeChecksums = true
//
// 	err := gopacket.SerializeLayers(buf, opts,
// 		&layers.Ethernet{
// 			SrcMAC:       host.MAC,
// 			DstMAC:       layers.EthernetBroadcast,
// 			EthernetType: layers.EthernetTypeARP,
// 		},
// 		&layers.ARP{
// 			AddrType:          layers.LinkTypeEthernet,
// 			Protocol:          layers.EthernetTypeIPv4,
// 			Operation:         layers.ARPRequest,
// 			SourceHwAddress:   host.MAC,
// 			SourceProtAddress: host.IPv4,
// 			DstHwAddress:      make(net.HardwareAddr, 6),
// 			DstProtAddress:    host.IPv4,
// 		})
// 	if err != nil {
// 		log.Printf("error[ARP]: %s", err)
// 		return
// 	}
//
// 	// opkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.NoCopy)
// 	// log.Printf("WRITE: %08x %s\n", 0, opkt.Dump())
//
// 	// _, err = vnet.iface.WritePacket(buf.Bytes(), 0)
// 	// if err != nil {
// 	// 	log.Printf("error[ARP]: %s", err)
// 	// 	return
// 	// }
//
// }
