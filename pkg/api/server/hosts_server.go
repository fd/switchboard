package server

import (
	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/hosts"
	"golang.org/x/net/context"
)

var _ protocol.HostsServer = (*hostsServer)(nil)

type hostsServer struct {
	hosts *hosts.Controller
}

func (s *hostsServer) List(context.Context, *protocol.HostListReq) (*protocol.HostListRes, error) {
	tab := s.hosts.GetTable()

	req := &protocol.HostListRes{}
	req.Hosts = make([]*protocol.Host, len(tab.Hosts()))
	for i, h := range tab.Hosts() {
		x := &protocol.Host{
			Id:   h.ID,
			Name: h.Name,
			Mac:  h.MAC.String(),
			Ipv4: make([]string, len(h.IPv4Addrs)),
			Ipv6: make([]string, len(h.IPv6Addrs)),
			Up:   h.Up,
		}

		for i, ip := range h.IPv4Addrs {
			x.Ipv4[i] = ip.String()
		}
		for i, ip := range h.IPv6Addrs {
			x.Ipv6[i] = ip.String()
		}

		req.Hosts[i] = x
	}

	return req, nil
}

func (s *hostsServer) Add(ctx context.Context, req *protocol.HostAddReq) (*protocol.HostAddRes, error) {
	host := &hosts.Host{}
	host.Name = req.Name

	if req.AllocateIPv4 {
		// TODO: allocate IPv4
	}

	host, err := s.hosts.AddHost(host)
	if err != nil {
		return nil, err
	}

	x := &protocol.Host{
		Id:   host.ID,
		Name: host.Name,
		Mac:  host.MAC.String(),
		Ipv4: make([]string, len(host.IPv4Addrs)),
		Ipv6: make([]string, len(host.IPv6Addrs)),
		Up:   host.Up,
	}

	for i, ip := range host.IPv4Addrs {
		x.Ipv4[i] = ip.String()
	}
	for i, ip := range host.IPv6Addrs {
		x.Ipv6[i] = ip.String()
	}

	res := &protocol.HostAddRes{
		Host: x,
	}

	return res, nil
}

func (s *hostsServer) Remove(ctx context.Context, req *protocol.HostRemoveReq) (*protocol.HostRemoveRes, error) {
	err := s.hosts.RemoveHost(req.Id)
	if err != nil {
		return nil, err
	}

	return &protocol.HostRemoveRes{}, nil
}
