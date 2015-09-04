package dispatcher

import (
	"log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"golang.org/x/net/context"
)

func (vnet *VNET) dispatchICMP(ctx context.Context) chan<- *Packet {
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

			vnet.handleICMP(pkt)
		}
	}()

	return in
}

func (vnet *VNET) handleICMP(pkt *Packet) {
	defer pkt.Release()

	if pkt.ICMPv4 != nil {
		vnet.handleICMPv4(pkt)
	} else if pkt.ICMPv6 != nil {
		// vnet.handleICMPv6(pkt)
	}
}

func (vnet *VNET) handleICMPv4(pkt *Packet) {
	// fmt.Printf("ICMP: %08x %s\n", pkt.Flags, pkt.String())

	switch uint8(pkt.ICMPv4.TypeCode >> 8) {
	case layers.ICMPv4TypeEchoRequest:
		log.Printf("ICMP/ping/req: %s -> %s\n", pkt.IPv4.SrcIP, pkt.IPv4.DstIP)
		vnet.handleICMPv4EchoRequest(pkt)
	default:
		log.Printf("ICMPv4/error: unkown type: %s\n", pkt.ICMPv4.TypeCode)
	}
}

func (vnet *VNET) handleICMPv4EchoRequest(pkt *Packet) {
	controller := vnet.hosts.GetTable().LookupByName("controller")
	if controller == nil {
		log.Printf("ICMPv4: controller is down\n")
		return
	}

	host := vnet.hosts.GetTable().LookupByIPv4(pkt.IPv4.DstIP)
	if host == nil {
		log.Printf("ICMPv4: DST: %s (unknown)\n", pkt.IPv4.DstIP)
		return
	}
	if !host.Up {
		log.Printf("ICMPv4: DST: %s (down)\n", pkt.IPv4.DstIP)
		return
	}
	if host.IPv4 == nil {
		log.Printf("ICMPv4: DST: %s (unknown)\n", pkt.IPv4.DstIP)
		return
	}
	log.Printf("ICMPv4: DST: %s (up)\n", pkt.IPv4.DstIP)

	buf := gopacket.NewSerializeBuffer()

	opts := gopacket.SerializeOptions{}
	opts.FixLengths = true
	opts.ComputeChecksums = true

	err := gopacket.SerializeLayers(buf, opts,
		&layers.Ethernet{
			SrcMAC:       controller.MAC,
			DstMAC:       pkt.Eth.SrcMAC,
			EthernetType: layers.EthernetTypeIPv4,
		},
		&layers.IPv4{
			SrcIP:    host.IPv4,
			DstIP:    pkt.IPv4.SrcIP,
			Version:  4,
			Protocol: layers.IPProtocolICMPv4,
			TTL:      64,
		},
		&layers.ICMPv4{
			TypeCode: layers.ICMPv4TypeEchoReply << 8,
			Seq:      pkt.ICMPv4.Seq,
			Id:       pkt.ICMPv4.Id,
			BaseLayer: layers.BaseLayer{
				Contents: pkt.ICMPv4.Contents,
			},
		},
		gopacket.Payload(pkt.ICMPv4.Payload))
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
}
