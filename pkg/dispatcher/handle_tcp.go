package dispatcher

import (
	"bytes"
	"log"
	"net"
	"time"

	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/routes"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"golang.org/x/net/context"
)

func (vnet *VNET) dispatchTCP(ctx context.Context) chan<- *Packet {
	var in = make(chan *Packet)

	vnet.wg.Add(1)
	go func() {
		defer vnet.wg.Done()

		var (
			now    time.Time
			ticker = time.NewTicker(1 * time.Second)
		)

		defer ticker.Stop()

		for {
			select {
			case now = <-ticker.C:
			case pkt := <-in:
				vnet.handleTCP(pkt, now)
			case <-ctx.Done():
				return
			}
		}
	}()

	return in
}

func (vnet *VNET) handleTCP(pkt *Packet, now time.Time) {
	// fmt.Printf("TCP: %08x %s\n", pkt.Flags, pkt.String())

	defer pkt.Release()

	var err error

	if bytes.Equal(pkt.Eth.DstMAC, layers.EthernetBroadcast[:]) {
		// ignore
		return
	}

	if pkt.DstHost == nil || !pkt.DstHost.Up {
		log.Printf("destination is down: %s", pkt.Eth.DstMAC)
		// ignore
		return
	}

	var (
		srcIP   net.IP
		dstIP   net.IP
		srcPort = uint16(pkt.TCP.SrcPort)
		dstPort = uint16(pkt.TCP.DstPort)
	)

	if pkt.IPv4 != nil {
		srcIP = CloneIP(pkt.IPv4.SrcIP.To16())
		dstIP = CloneIP(pkt.IPv4.DstIP.To16())
	} else if pkt.IPv6 != nil {
		srcIP = CloneIP(pkt.IPv6.SrcIP.To16())
		dstIP = CloneIP(pkt.IPv6.DstIP.To16())
	} else {
		log.Printf("invalid protocol")
		// ignore
		return
	}

	route := vnet.routes.GetTable().Lookup(
		protocols.TCP,
		srcIP, dstIP, srcPort, dstPort)

	if route == nil {
		rule, found := vnet.rules.GetTable().Lookup(protocols.TCP, pkt.DstHost.ID, dstPort)
		if !found {
			log.Printf("no rule")
			// ignore
			return
		}

		var ruleDstIP = rule.DstIP

		if ruleDstIP == nil {
			gateway := vnet.hosts.GetTable().LookupByName("gateway")
			if gateway == nil || !gateway.Up {
				log.Printf("no gateway")
				// ignore
				return
			}

			if dstIP.To4() != nil {
				if len(gateway.IPv4Addrs) > 0 {
					ruleDstIP = gateway.IPv4Addrs[0]
				}
			} else {
				if len(gateway.IPv6Addrs) > 0 {
					ruleDstIP = gateway.IPv6Addrs[0]
				}
			}
		}
		if ruleDstIP == nil {
			log.Printf("no destination ip")
			// ignore
			return
		}

		var r routes.Route
		r.Protocol = protocols.TCP
		r.HostID = pkt.DstHost.ID
		r.SetInboundSource(srcIP, srcPort)
		r.SetInboundDestination(dstIP, dstPort)
		r.SetOutboundDestination(ruleDstIP, rule.DstPort)
		route, err = vnet.routes.AddRoute(&r)
		if err != nil {
			// ignore
			log.Printf("TCP/error: %s", err)
			return
		}
	}

	if route == nil {
		log.Printf("no route")
		// ignore
		return
	}

	var (
		eth layers.Ethernet
		tcp layers.TCP
		buf = gopacket.NewSerializeBuffer()
	)

	eth = *pkt.Eth
	eth.SrcMAC, eth.DstMAC = eth.DstMAC, eth.SrcMAC

	tcp = *pkt.TCP
	tcp.SrcPort = layers.TCPPort(route.Outbound.SrcPort)
	tcp.DstPort = layers.TCPPort(route.Outbound.DstPort)

	opts := gopacket.SerializeOptions{}
	opts.FixLengths = true
	opts.ComputeChecksums = true

	if route.Outbound.DstIP.To4() != nil {
		ip := layers.IPv4{
			SrcIP:    route.Outbound.SrcIP.To4(),
			DstIP:    route.Outbound.DstIP.To4(),
			Version:  4,
			Protocol: layers.IPProtocolTCP,
			TTL:      64,
		}

		tcp.SetNetworkLayerForChecksum(&ip)

		err = gopacket.SerializeLayers(buf, opts,
			&eth,
			&ip,
			&tcp,
			gopacket.Payload(pkt.TCP.Payload))
		if err != nil {
			log.Printf("TCP/error: %s", err)
			return
		}

	} else {
		ip := layers.IPv6{
			SrcIP:      route.Outbound.SrcIP.To16(),
			DstIP:      route.Outbound.DstIP.To16(),
			Version:    4,
			NextHeader: layers.IPProtocolTCP,
		}

		tcp.SetNetworkLayerForChecksum(&ip)

		err = gopacket.SerializeLayers(buf, opts,
			&eth,
			&ip,
			&tcp,
			gopacket.Payload(pkt.TCP.Payload))
		if err != nil {
			log.Printf("TCP/error: %s", err)
			return
		}
	}

	_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
	if err != nil {
		log.Printf("TCP/error: %s", err)
		return
	}

	route.RoutedPacket(now, len(pkt.buf))
}
