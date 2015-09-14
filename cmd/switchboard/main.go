package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"text/tabwriter"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/api/server"
	"github.com/fd/switchboard/pkg/dispatcher"
	"github.com/fd/switchboard/pkg/dns"
	"github.com/fd/switchboard/pkg/plugin/driver"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go quitSignal(cancel)
	defer cancel()

	app := kingpin.New("switchboard", "").Version("1.0a").Author("Simon Menke")

	daemon := app.Command("daemon", "run the daemon")
	hosts := app.Command("hosts", "list the hosts")
	addresses := app.Command("addresses", "list the routed addresses")

	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case daemon.FullCommand():
		runServer(ctx)
	case hosts.FullCommand():
		listHosts(ctx)
	case addresses.FullCommand():
		listAddresses(ctx)
	}
}

func runServer(ctx context.Context) {
	vnet, err := dispatcher.Run(ctx)
	assert(err)

	err = server.Run(ctx, vnet)
	assert(err)

	// export DOCKER_TLS_VERIFY="1"
	// export DOCKER_HOST="tcp://192.168.99.100:2376"
	// export DOCKER_CERT_PATH="/Users/fd/.docker/machine/machines/default"
	driver.Run(ctx, "docker", map[string]interface{}{
		"host":       "tcp://192.168.99.100:2376",
		"verify-tls": true,
		"cert-path":  "/Users/fd/.docker/machine/machines/default",
	})

	dns.Run(ctx, vnet)

	defer vnet.Wait()
}

func listHosts(ctx context.Context) {
	conn, err := grpc.Dial("172.18.0.1:8080")
	assert(err)
	defer conn.Close()

	client := protocol.NewHostsClient(conn)

	in := protocol.HostListReq{}
	out, err := client.List(ctx, &in)
	assert(err)

	tabw := tabwriter.NewWriter(os.Stdout, 8, 8, 2, ' ', 0)
	defer tabw.Flush()
	fmt.Fprintf(tabw, "%s\t%s\t%s\n", "ID", "NAME", "STATE")
	for _, host := range out.Hosts {
		state := "down"
		if host.Up {
			state = "up"
		}

		fmt.Fprintf(tabw, "%s\t%s\t%v\n", host.Id[:8], host.Name, state)
	}
}

func listAddresses(ctx context.Context) {
	conn, err := grpc.Dial("172.18.0.1:8080")
	assert(err)
	defer conn.Close()

	client := protocol.NewHostsClient(conn)

	in := protocol.HostListReq{}
	out, err := client.List(ctx, &in)
	assert(err)

	tabw := tabwriter.NewWriter(os.Stdout, 8, 8, 2, ' ', 0)
	defer tabw.Flush()
	fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\n", "HOST", "NAME", "IP", "VERSION")
	for _, host := range out.Hosts {
		sort.Strings(host.Ipv4)
		sort.Strings(host.Ipv6)

		for _, ip := range host.Ipv4 {
			fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\n", host.Id[:8], host.Name, ip, "ipv4")
		}
		for _, ip := range host.Ipv6 {
			fmt.Fprintf(tabw, "%s\t%s\t%s\t%s\n", host.Id[:8], host.Name, ip, "ipv6")
		}
	}
}

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

func quitSignal(cancel func()) {
	defer cancel()

	c := make(chan os.Signal)
	defer signal.Stop(c)
	go signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
