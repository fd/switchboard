package plugin

import (
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/hashicorp/hcl"
)

func init() {
	log.SetPrefix("")
	log.SetFlags(0)
}

type Plugin struct {
	conn   *grpc.ClientConn
	hosts  protocol.HostsClient
	rules  protocol.RulesClient
	config string
}

type HandlerFunc func(context.Context, *Plugin) error

func Run(handler HandlerFunc) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go quitSignal(cancel)
	defer cancel()

	u, err := url.Parse(os.Getenv("SWITCHBOARD_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if u.Scheme != "tcp" {
		log.Fatal("SWITCHBOARD_URL must start with tcp://")
	}

	conn, err := grpc.Dial(u.Host)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	plugin := &Plugin{
		conn:   conn,
		hosts:  protocol.NewHostsClient(conn),
		rules:  protocol.NewRulesClient(conn),
		config: os.Getenv("SWITCHBOARD_CONFIG"),
	}

	err = handler(ctx, plugin)
	if err != nil {
		log.Fatal(err)
	}
}

func (plugin *Plugin) Hosts() protocol.HostsClient {
	return plugin.hosts
}

func (plugin *Plugin) Rules() protocol.RulesClient {
	return plugin.rules
}

func (plugin *Plugin) ParseConfig(v interface{}) error {
	return hcl.Decode(v, plugin.config)
}

func quitSignal(cancel func()) {
	defer cancel()

	c := make(chan os.Signal)
	defer signal.Stop(c)
	go signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
