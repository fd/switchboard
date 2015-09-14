package dns

import (
	"log"
	"net"
	"strings"

	"github.com/fd/switchboard/pkg/dispatcher"
	"github.com/fd/switchboard/pkg/hosts"
	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/rules"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func Run(ctx context.Context, vnet *dispatcher.VNET) <-chan error {
	out := make(chan error, 1)
	go func() {
		defer close(out)

		system := vnet.System()
		system.WaitForGatewayIPv4()

		l, err := net.ListenUDP("udp", &net.UDPAddr{IP: system.GatewayIPv4()})
		if err != nil {
			log.Printf("error=%s", err)
			return
		}

		_, err = vnet.Rules().AddRule(rules.Rule{
			DstPort:   uint16(l.LocalAddr().(*net.UDPAddr).Port),
			Protocol:  protocols.UDP,
			SrcHostID: vnet.Hosts().GetTable().LookupByName("controller").ID,
			SrcPort:   53,
		})
		if err != nil {
			log.Printf("error=%s", err)
			return
		}

		server := &dns.Server{
			Net:        "udp",
			Handler:    &handler{vnet: vnet},
			PacketConn: l,
		}

		err = server.ActivateAndServe()
		if err != nil {
			log.Printf("error=%s", err)
		}
	}()
	return out
}

type handler struct {
	vnet *dispatcher.VNET
}

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	defer w.WriteMsg(m)

	if len(r.Question) != 1 {
		return
	}

	q := r.Question[0]
	if q.Qclass != dns.ClassINET {
		return
	}
	if q.Qtype != dns.TypeA {
		return
	}

	withID := false
	labels := dns.SplitDomainName(q.Name)
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	if len(labels) > 0 && labels[0] == "" {
		labels = labels[1:]
	}
	if len(labels) > 0 && labels[0] == "switch" {
		labels = labels[1:]
	}
	if len(labels) > 0 && labels[0] == "id" {
		labels = labels[1:]
		withID = true
	}
	name := strings.Join(labels, "/")

	var host *hosts.Host
	if withID {
		host = h.vnet.Hosts().GetTable().LookupByID(name)
	} else {
		host = h.vnet.Hosts().GetTable().LookupByName(name)
	}
	if host == nil {
		return
	}

	for _, ip := range host.IPv4Addrs {
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: ip,
		})
	}
}
