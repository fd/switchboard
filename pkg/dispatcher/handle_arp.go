package dispatcher

import (
	"bytes"
	"log"
	"net"
	"time"

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
				vnet.handleARPReply(pkt)

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

	if !bytes.Equal(pkt.Eth.DstMAC, layers.EthernetBroadcast[:]) {
		// ignore; expect broadcast
		return
	}

	if pkt.ARP.ProtAddressSize != 4 {
		// ignore; expect ipv4
		return
	}

	if bytes.Equal(pkt.ARP.SourceProtAddress, pkt.ARP.DstProtAddress) {
		if vnet.system.GatewayMAC() == nil {
			vnet.system.SetGatewayMAC(pkt.ARP.SourceHwAddress)
			vnet.system.SetGatewayIPv4(pkt.ARP.SourceProtAddress)
		}

		// ignore; announce
		return
	}

	if vnet.system.ControllerMAC() == nil {
		return
	}
	if vnet.system.ControllerIPv4() == nil {
		return
	}
	if vnet.system.GatewayMAC() == nil {
		return
	}
	if vnet.system.GatewayIPv4() == nil {
		return
	}
	if !bytes.Equal(pkt.ARP.DstProtAddress, vnet.system.ControllerIPv4()) {
		return
	}

	eth := layers.Ethernet{
		SrcMAC:       vnet.system.ControllerMAC(),
		DstMAC:       vnet.system.GatewayMAC(),
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		DstHwAddress:      pkt.ARP.SourceHwAddress,
		DstProtAddress:    pkt.ARP.SourceProtAddress,
		SourceHwAddress:   vnet.system.ControllerMAC(),
		SourceProtAddress: vnet.system.ControllerIPv4(),
		Operation:         layers.ARPReply,
	}

	vnet.writePacket(&eth, &arp)
}

func (vnet *VNET) handleARPReply(pkt *Packet) {
	defer pkt.Release()
	log.Printf("ARP REP: %08x %s\n", pkt.Flags, pkt.Dump())

	vnet.peers.AddPeer(
		CloneIP(pkt.ARP.SourceProtAddress),
		CloneHwAddress(pkt.ARP.SourceHwAddress))
}

func (vnet *VNET) lookupHarwareAddrForIP(ip net.IP) net.HardwareAddr {
	addr := vnet.peers.Lookup(ip)

	for i := 0; i < 3 && addr == nil; i++ {
		vnet.sendARPReuest(ip)

		for i := 0; i < 100 && addr == nil; i++ {
			time.Sleep(10 * time.Millisecond)
			addr = vnet.peers.Lookup(ip)
		}
	}

	return addr
}

func (vnet *VNET) sendARPReuest(ip net.IP) {
	eth := layers.Ethernet{
		SrcMAC:       vnet.system.ControllerMAC(),
		DstMAC:       layers.EthernetBroadcast,
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		DstHwAddress:      make([]byte, 6),
		DstProtAddress:    ip.To4(),
		SourceHwAddress:   vnet.system.ControllerMAC(),
		SourceProtAddress: vnet.system.ControllerIPv4(),
		Operation:         layers.ARPRequest,
	}

	vnet.writePacket(&eth, &arp)
}
