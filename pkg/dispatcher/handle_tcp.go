package dispatcher

import (
	"time"

	"github.com/fd/switchboard/pkg/rules"

	"golang.org/x/net/context"
)

type tcpEndpoint struct {
	ip   [16]byte
	port uint16
}

type tcpFlow struct {
	Dst    tcpEndpoint
	Expire time.Time
}

func (vnet *VNET) dispatchTCP(ctx context.Context) chan<- *Packet {
	var in = make(chan *Packet)

	var flows = map[tcpEndpoint]*tcpFlow{}

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

			if pkt.DstHost == nil || !pkt.DstHost.Up {
				// ignore
				pkt.Release()
				continue
			}

			var src tcpEndpoint

			// get src-(ext|int) endpoint
			if pkt.IPv4 != nil {
				copy(src.ip[:], pkt.IPv4.SrcIP.To16())
				src.port = uint16(pkt.TCP.SrcPort)
			} else if pkt.IPv6 != nil {
				copy(src.ip[:], pkt.IPv4.SrcIP.To16())
				src.port = uint16(pkt.TCP.SrcPort)
			} else {
				// ignore
				pkt.Release()
				continue
			}

			flow := flows[src]
			if flow == nil {
				// flows:
				// src-ext-ip:src-ext-port -> dst-int-ip:dst-int-port
				// src-int-ip:src-int-port -> dst-ext-ip:dst-ext-port
				//
				// src-ext: packets flowing in through the dispatcher from the initiator
				// dst-int: the target of the rule

				var (
					srcExt = src
					dstInt tcpEndpoint
					srcInt tcpEndpoint
					dstExt tcpEndpoint
				)

				rule, found := vnet.rules.GetTable().Lookup(rules.TCP, pkt.DstHost.ID, uint16(pkt.TCP.DstPort))
				if !found {
					// ignore
					pkt.Release()
					continue
				}

				// get dst-int endpoint
				if rule.DstIPv4 != nil {
					copy(dst.ip[:], rule.DstIPv4.To16())
					dst.port = uint16(rule.DstPort)
				} else if rule.DstIPv6 != nil {
					copy(dst.ip[:], pkt.DstIPv6.To16())
					dst.port = uint16(rule.DstPort)
				} else {
					// ignore
					pkt.Release()
					continue
				}

				// get src-int endpoint
				if pkt.IPv4 != nil {
					copy(srcInt.ip[:], pkt.IPv4.SrcIP.To16())
					srcInt.port = uint16(pkt.TCP.SrcPort)
				} else if pkt.IPv6 != nil {
					copy(srcInt.ip[:], pkt.IPv4.SrcIP.To16())
					srcInt.port = uint16(pkt.TCP.SrcPort)
				} else {
					// ignore
					pkt.Release()
					continue
				}

				// get dst-in endpoint
				if rule.DstIPv4 != nil {
					copy(dst.ip[:], rule.DstIPv4.To16())
					dst.port = uint16(rule.DstPort)
				} else if rule.DstIPv6 != nil {
					copy(dst.ip[:], pkt.DstIPv6.To16())
					dst.port = uint16(rule.DstPort)
				} else {
					// ignore
					pkt.Release()
					continue
				}

			}

			// 1. lookup flow
			// 2. if flow == nil
			//    1. lookup rule
			//    2. make flow and reverse-flow
			// 3. forward packet

			// fmt.Printf("TCPv4: %08x %s\n", pkt.Flags, pkt.String())
		}
	}()

	return in
}
