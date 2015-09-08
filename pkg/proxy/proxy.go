package proxy

import (
	"sync"

	"github.com/fd/switchboard/pkg/routes"
	"golang.org/x/net/context"
)

type Proxy struct {
	TCPPort uint16
	UPPort  uint16

	routes *routes.Controller
	wg     sync.WaitGroup
}

func NewProxy(routes *routes.Controller) *Proxy {
	return &Proxy{
		routes: routes,
	}
}

func (p *Proxy) Run(ctx context.Context) error {
	ctx, _ = context.WithCancel(ctx)
	// defer cancel()

	err := p.proxyTCP(ctx)
	if err != nil {
		return err
	}

	return nil
}
