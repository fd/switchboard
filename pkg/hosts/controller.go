package hosts

import (
	"errors"
	"net"
	"sync"

	"github.com/dustinkirkland/golang-petname"
	"github.com/satori/go.uuid"
)

type Controller struct {
	mtx   sync.Mutex
	hosts map[string]*Host

	tableMtx sync.RWMutex
	table    *Table
}

func NewController() *Controller {
	return &Controller{
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
	if host.MAC != nil && tab.LookupByMAC(host.MAC) != nil {
		return nil, errors.New("host MAC is already in use")
	}
	if host.IPv4 != nil && tab.LookupByIPv4(host.IPv4) != nil {
		return nil, errors.New("host IPv4 is already in use")
	}
	if host.IPv6 != nil && tab.LookupByIPv6(host.IPv6) != nil {
		return nil, errors.New("host IPv6 is already in use")
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
	if host.MAC == nil {
		for {
			mac, err := generateMAC()
			if err != nil {
				return nil, err
			}
			if tab.LookupByMAC(mac) == nil {
				host.MAC = mac
				break
			}
		}
	}
	if host.IPv6 == nil {
		for {
			ip, err := generateIPv6(host.Local)
			if err != nil {
				return nil, err
			}
			if tab.LookupByIPv6(ip) == nil {
				host.IPv6 = ip
				break
			}
		}
	}

	if host.IPv4 != nil {
		host.IPv4 = host.IPv4.To4()
	}

	if host.IPv6 != nil {
		host.IPv6 = host.IPv6.To16()
	}

	c.hosts[host.ID] = host
	c.updateTable()

	return host, nil
}

func (c *Controller) RemoveHost(id string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	host := c.lookupByNameOrID(id)
	delete(c.hosts, host.ID)
	c.updateTable()
	return nil
}

func (c *Controller) HostSetIPv4(id string, ip net.IP) error {
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
		host.IPv4 = ip.To4()
	} else {
		host.IPv4 = nil
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
