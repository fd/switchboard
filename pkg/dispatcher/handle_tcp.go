package dispatcher

import (
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

		for {
			var (
				pkt *Packet
				err error
			)

			select {
			case pkt = <-in:
			case <-ctx.Done():
				return
			}

			if pkt.DstHost == nil || !pkt.DstHost.Up {
				// ignore
				pkt.Release()
				continue
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
				// ignore
				pkt.Release()
				continue
			}

			route := vnet.routes.GetTable().Lookup(
				protocols.TCP,
				srcIP, dstIP, srcPort, dstPort)

			if route == nil {
				rule, found := vnet.rules.GetTable().Lookup(protocols.TCP, pkt.DstHost.ID, dstPort)
				if !found {
					// ignore
					pkt.Release()
					continue
				}

				var r routes.Route
				r.Protocol = protocols.TCP
				r.HostID = pkt.DstHost.ID
				r.SetInboundSource(srcIP, srcPort)
				r.SetInboundDestination(dstIP, dstPort)
				r.SetOutboundDestination(rule.DstIP, rule.DstPort)
				route, err = vnet.routes.AddRoute(&r)
				if err != nil {
					// ignore
					pkt.Release()
					continue
				}
			}

			if route == nil {
				// ignore
				pkt.Release()
				continue
			}

			var (
				eth  layers.Ethernet
				ipv4 layers.IPv4
				ipv6 layers.IPv6
				tcp  layers.TCP
				ip   gopacket.NetworkLayer
			)

			eth = *pkt.Eth
			eth.SrcMAC, eth.DstMAC = eth.DstMAC, eth.SrcMAC

			if route.Outbound.DstIP.To4() != nil {
				ipv4 = layers.IPv4{
					SrcIP:    route.Outbound.SrcIP.To4(),
					DstIP:    route.Outbound.DstIP.To4(),
					Version:  4,
					Protocol: layers.IPProtocolTCP,
					TTL:      64,
				}
				ip = &ipv4
			} else {
				ipv6 = layers.IPv6{
					SrcIP:    route.Outbound.SrcIP.To16(),
					DstIP:    route.Outbound.DstIP.To16(),
					Version:  4,
					Protocol: layers.IPProtocolTCP,
					TTL:      64,
				}
				ip = &ipv6
			}

			tcp = *pkt.TCP
			tcp.SrcPort = layers.TCPPort(route.Outbound.SrcPort)
			tcp.DstPort = layers.TCPPort(route.Outbound.DstPort)
			tcp.SetNetworkLayerForChecksum(ip)

			buf := gopacket.NewSerializeBuffer()

			opts := gopacket.SerializeOptions{}
			opts.FixLengths = true
			opts.ComputeChecksums = true

			err = gopacket.SerializeLayers(buf, opts,
				&eth,
				ip,
				&tcp,
				gopacket.Payload(pkt.TCP.Payload))
			if err != nil {
				log.Printf("TCP/error: %s", err)
				return
			}

			_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
			if err != nil {
				log.Printf("TCP/error: %s", err)
				return
			}

			route.RoutedPacket(time.Now(), len(pkt.buf))
		}
	}()

	return in
}
