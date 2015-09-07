package server

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/dispatcher"
	"github.com/fd/switchboard/pkg/hosts"
	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/rules"
)

func Run(ctx context.Context, vnet *dispatcher.VNET) error {
	var (
		port       int
		gateway    *hosts.Host
		controller *hosts.Host
	)

	for controller == nil || len(controller.IPv4Addrs) == 0 {
		controller = vnet.Hosts().GetTable().LookupByName("controller")
		time.Sleep(100 * time.Millisecond)
	}

	for gateway == nil || len(gateway.IPv4Addrs) == 0 {
		gateway = vnet.Hosts().GetTable().LookupByName("gateway")
		time.Sleep(100 * time.Millisecond)
	}

	l, err := net.Listen("tcp", gateway.IPv4Addrs[0].String()+":0")
	if err != nil {
		return err
	}

	port = l.Addr().(*net.TCPAddr).Port
	_, err = vnet.Rules().AddRule(rules.Rule{
		Protocol:  protocols.TCP,
		SrcHostID: controller.ID,
		SrcPort:   8080,
		DstPort:   uint16(port),
	})
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	protocol.RegisterHostsServer(grpcServer, &hostsServer{hosts: vnet.Hosts()})

	for _, ip := range controller.IPv4Addrs {
		log.Printf("API: %s:%d (external)", ip.String(), 8080)
	}
	log.Printf("API: %s:%d (internal)", "127.0.0.1", port)

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	go func() {
		err := grpcServer.Serve(l)
		if err != nil {
			log.Printf("API/error: %s", err)
		}
	}()

	return nil
}
