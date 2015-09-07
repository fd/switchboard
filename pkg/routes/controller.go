package routes

import (
	"errors"
	"sync"
	"time"

	"github.com/fd/switchboard/pkg/ports"
)

type Controller struct {
	ports *ports.Mapper

	mtx    sync.Mutex
	routes []*Route

	tableMtx sync.RWMutex
	table    *Table
}

func NewController(ports *ports.Mapper) *Controller {
	return &Controller{
		ports: ports,
		table: &Table{},
	}
}

func (c *Controller) GetTable() *Table {
	c.tableMtx.RLock()
	defer c.tableMtx.RUnlock()

	return c.table
}

func (c *Controller) AddRoute(route *Route) (*Route, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	tab := c.GetTable()
	route = route.Clone()

	if !route.Protocol.Valid() {
		return nil, errors.New("route protocol is invalid")
	}
	if route.HostID == "" {
		return nil, errors.New("route host ID must be set")
	}
	if route.Inbound.SrcIP == nil {
		return nil, errors.New("route inbound source IP must be set")
	}
	if route.Inbound.SrcPort == 0 {
		return nil, errors.New("route inbound source IP must be set")
	}
	if route.Inbound.DstIP == nil {
		return nil, errors.New("route inbound destination IP must be set")
	}
	if route.Inbound.DstPort == 0 {
		return nil, errors.New("route inbound destination IP must be set")
	}
	if route.Outbound.DstIP == nil {
		return nil, errors.New("route outbound destination IP must be set")
	}
	if route.Outbound.DstPort == 0 {
		return nil, errors.New("route outbound destination IP must be set")
	}

	if route.Outbound.SrcIP == nil {
		route.Outbound.SrcIP = route.Inbound.DstIP
	}
	if route.Inbound.SrcIP != nil {
		route.Inbound.SrcIP = route.Inbound.SrcIP.To16()
	}
	if route.Inbound.DstIP != nil {
		route.Inbound.DstIP = route.Inbound.DstIP.To16()
	}
	if route.Outbound.SrcIP != nil {
		route.Outbound.SrcIP = route.Outbound.SrcIP.To16()
	}
	if route.Outbound.DstIP != nil {
		route.Outbound.DstIP = route.Outbound.DstIP.To16()
	}

	if p, err := c.ports.Allocate(route.HostID, route.Protocol, route.Outbound.SrcPort); err == nil {
		route.Outbound.SrcPort = p
	} else {
		return nil, err
	}

	if tab.Lookup(route.Protocol,
		route.Inbound.SrcIP, route.Inbound.DstIP,
		route.Inbound.SrcPort, route.Inbound.DstPort) != nil {
		c.ports.Release(route.HostID, route.Protocol, route.Outbound.SrcPort)
		return nil, errors.New("route already exists")
	}
	if tab.Lookup(route.Protocol,
		route.Outbound.DstIP, route.Outbound.SrcIP,
		route.Outbound.DstPort, route.Outbound.SrcPort) != nil {
		c.ports.Release(route.HostID, route.Protocol, route.Outbound.SrcPort)
		return nil, errors.New("route already exists")
	}

	route.buildFlow()

	c.routes = append(c.routes, route)
	c.routes = append(c.routes, route.reverse())
	c.updateTable()

	return route, nil
}

func (c *Controller) Expire() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	now := time.Now()

	routes := make([]*Route, 0, len(c.routes))
	for _, route := range c.routes {
		if !route.flow.Expired(now) {
			routes = append(routes, route)
		}
	}

	c.routes = routes
	c.updateTable()
}

func (c *Controller) updateTable() {
	tab := buildTable(c.routes)

	c.tableMtx.Lock()
	c.table = tab
	c.tableMtx.Unlock()
}
