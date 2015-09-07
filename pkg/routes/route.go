package routes

import (
	"bytes"
	"net"
	"time"

	"github.com/fd/switchboard/pkg/protocols"
)

type Route struct {
	Protocol protocols.Protocol
	HostID   string

	Inbound struct {
		SrcIP   net.IP
		SrcPort uint16

		DstIP   net.IP
		DstPort uint16
	}

	Outbound struct {
		SrcIP   net.IP
		SrcPort uint16

		DstIP   net.IP
		DstPort uint16
	}

	flow *Flow
}

func (r *Route) SetInboundSource(ip net.IP, port uint16) {
	r.Inbound.SrcIP = ip
	r.Inbound.SrcPort = port
}

func (r *Route) SetInboundDestination(ip net.IP, port uint16) {
	r.Inbound.DstIP = ip
	r.Inbound.DstPort = port
}

func (r *Route) SetOutboundSource(ip net.IP, port uint16) {
	r.Outbound.SrcIP = ip
	r.Outbound.SrcPort = port
}

func (r *Route) SetOutboundDestination(ip net.IP, port uint16) {
	r.Outbound.DstIP = ip
	r.Outbound.DstPort = port
}

func (r *Route) RoutedPacket(now time.Time, size int) {
	if r == r.flow.rxRoute {
		r.flow.receivedPacket(now, uint64(size))
	} else {
		r.flow.sendPacket(now, uint64(size))
	}
}

func (r *Route) Clone() *Route {
	clone := new(Route)
	*clone = *r
	clone.flow = nil
	return clone
}

func (r *Route) buildFlow() *Flow {
	if r.flow != nil {
		return r.flow
	}

	flow := &Flow{}
	flow.rxRoute = r
	flow.txRoute = r.reverse()
	flow.rxRoute.flow = flow
	flow.txRoute.flow = flow

	flow.timeout = 55 // 55 seconds
	flow.touch(time.Now())

	return flow
}

func (r *Route) reverse() *Route {
	if r.flow != nil {
		if r.flow.rxRoute == r {
			return r.flow.txRoute
		}
		return r.flow.rxRoute
	}

	reverse := &Route{}
	reverse.Protocol = r.Protocol

	reverse.SetInboundSource(r.Outbound.DstIP, r.Outbound.DstPort)
	reverse.SetInboundDestination(r.Outbound.SrcIP, r.Outbound.SrcPort)

	reverse.SetOutboundSource(r.Inbound.DstIP, r.Inbound.DstPort)
	reverse.SetOutboundDestination(r.Inbound.SrcIP, r.Inbound.SrcPort)

	return reverse
}

func lessInbound(a, b *Route) bool {
	if a.Protocol < b.Protocol {
		return true
	}
	if a.Protocol > b.Protocol {
		return false
	}

	if c := bytes.Compare(a.Inbound.SrcIP, b.Inbound.SrcIP); c < 0 {
		return true
	} else if c > 0 {
		return false
	}
	if a.Inbound.SrcPort < b.Inbound.SrcPort {
		return true
	}
	if a.Inbound.SrcPort > b.Inbound.SrcPort {
		return false
	}

	if c := bytes.Compare(a.Inbound.DstIP, b.Inbound.DstIP); c < 0 {
		return true
	} else if c > 0 {
		return false
	}
	if a.Inbound.DstPort < b.Inbound.DstPort {
		return true
	}
	if a.Inbound.DstPort > b.Inbound.DstPort {
		return false
	}

	return false
}
