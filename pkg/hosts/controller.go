package hosts

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/dustinkirkland/golang-petname"
	"github.com/fd/switchboard/pkg/ports"
	"github.com/satori/go.uuid"
)

type Controller struct {
	ports *ports.Mapper

	mtx   sync.Mutex
	hosts map[string]*Host

	tableMtx sync.RWMutex
	table    *Table
}

func NewController(ports *ports.Mapper) *Controller {
	return &Controller{
		ports: ports,
		hosts: make(map[string]*Host),
		table: &Table{},
	}
}

func (c *Controller) GetTable() *Table {
	c.tableMtx.RLock()
	defer c.tableMtx.RUnlock()

	return c.table
}

func (c *Controller) AddHost(host *Host) (*Host, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	tab := c.GetTable()
	host = host.Clone()

	if host.ID != "" && tab.LookupByID(host.ID) != nil {
		return nil, errors.New("host id is already in use")
	}
	if host.Name != "" && tab.LookupByName(host.Name) != nil {
		return nil, errors.New("host name is already in use")
	}
	for _, ip := range host.IPv4Addrs {
		if tab.LookupByIPv4(ip) != nil {
			return nil, fmt.Errorf("host IPv4 %s is already in use", ip)
		}
	}
	for _, ip := range host.IPv6Addrs {
		if tab.LookupByIPv6(ip) != nil {
			return nil, fmt.Errorf("host IPv6 %s is already in use", ip)
		}
	}

	if host.ID == "" {
		for {
			id := uuid.NewV4().String()
			if tab.LookupByID(id) == nil {
				host.ID = id
				break
			}
		}
	}
	if host.Name == "" {
		for {
			name := petname.Generate(2, "-")
			if tab.LookupByName(name) == nil {
				host.Name = name
				break
			}
		}
	}
	if len(host.IPv6Addrs) == 0 {
		for {
			ip, err := generateIPv6(host.Local)
			if err != nil {
				return nil, err
			}
			if tab.LookupByIPv6(ip) == nil {
				host.IPv6Addrs = append(host.IPv6Addrs, ip)
				break
			}
		}
	}

	for i, ip := range host.IPv4Addrs {
		host.IPv4Addrs[i] = ip.To4()
	}

	for i, ip := range host.IPv6Addrs {
		host.IPv6Addrs[i] = ip.To16()
	}

	c.hosts[host.ID] = host
	c.updateTable()

	return host, nil
}

func (c *Controller) RemoveHost(id string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.ports.ForgetHost(id)

	host := c.lookupByNameOrID(id)
	delete(c.hosts, host.ID)
	c.updateTable()
	return nil
}

func (c *Controller) HostAddIPv4(id string, ip net.IP) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	var (
		tab  = c.GetTable()
		host = c.lookupByNameOrID(id)
	)

	if host == nil {
		return errors.New("host not found")
	}
	if h := tab.LookupByIPv4(ip); h != nil && h != host {
		return errors.New("host IPv4 is already in use")
	}

	if ip != nil {
		host.IPv4Addrs = append(host.IPv4Addrs, ip.To4())
	} else {
		// host.IPv4 = nil
	}

	c.updateTable()

	return nil
}

func (c *Controller) HostSetState(id string, up bool) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	host := c.lookupByNameOrID(id)
	if host == nil {
		return errors.New("host not found")
	}

	if !up {
		c.ports.ForgetHost(id)
	}

	host.Up = up
	c.updateTable()

	return nil
}

func (c *Controller) lookupByNameOrID(id string) *Host {
	h := c.GetTable().LookupByNameOrID(id)
	if h == nil {
		return nil
	}

	h = c.hosts[h.ID]
	if h == nil {
		return nil
	}

	return h
}

func (c *Controller) updateTable() {
	hosts := make([]*Host, 0, len(c.hosts))
	for _, h := range c.hosts {
		hosts = append(hosts, h.Clone())
	}
	tab := buildTable(hosts)

	c.tableMtx.Lock()
	c.table = tab
	c.tableMtx.Unlock()
}
