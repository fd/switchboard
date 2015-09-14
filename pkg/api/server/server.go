package server

import (
	"log"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/dispatcher"
	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/rules"
)

func Run(ctx context.Context, vnet *dispatcher.VNET) error {
	var (
		port int
	)

	vnet.System().WaitForControllerIPv4()
	vnet.System().WaitForGatewayIPv4()

	l, err := net.Listen("tcp", vnet.System().GatewayIPv4().String()+":0")
	if err != nil {
		return err
	}

	controller := vnet.Hosts().GetTable().LookupByName("controller")

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
	protocol.RegisterRulesServer(grpcServer, &rulesServer{rules: vnet.Rules()})

	for _, ip := range controller.IPv4Addrs {
		log.Printf("API: %s:%d (external)", ip.String(), 8080)
	}
	log.Printf("API: %s:%d (internal)", vnet.System().GatewayIPv4(), port)

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
