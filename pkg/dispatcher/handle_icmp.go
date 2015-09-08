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

		vnet.system.WaitForControllerMAC()
		log.Printf("ICMP: running")

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
	host := vnet.hosts.GetTable().LookupByIPv4(pkt.IPv4.DstIP)
	if host == nil {
		log.Printf("ICMPv4: DST: %s (unknown)\n", pkt.IPv4.DstIP)
		return
	}
	if !host.Up {
		log.Printf("ICMPv4: DST: %s (down)\n", pkt.IPv4.DstIP)
		return
	}
	if len(host.IPv4Addrs) == 0 {
		log.Printf("ICMPv4: DST: %s (unknown)\n", pkt.IPv4.DstIP)
		return
	}
	log.Printf("ICMPv4: DST: %s (up)\n", pkt.IPv4.DstIP)

	err := vnet.writePacket(
		&layers.Ethernet{
			SrcMAC:       vnet.system.ControllerMAC(),
			DstMAC:       pkt.Eth.SrcMAC,
			EthernetType: layers.EthernetTypeIPv4,
		},
		&layers.IPv4{
			SrcIP:    pkt.IPv4.DstIP,
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
}
