package hosts

import (
	"bytes"
	"fmt"
	"net"
	"sort"
)

type Table struct {
	id   []*Host
	name []*Host
	mac  []*Host
	ipv4 []*Host
	ipv6 []*Host
}

func buildTable(hosts []*Host) *Table {
	tab := &Table{
		id:   make([]*Host, len(hosts)),
		name: make([]*Host, len(hosts)),
		mac:  make([]*Host, len(hosts)),
		ipv4: make([]*Host, len(hosts)),
		ipv6: make([]*Host, len(hosts)),
	}

	copy(tab.id, hosts)
	copy(tab.name, hosts)
	copy(tab.mac, hosts)
	copy(tab.ipv4, hosts)
	copy(tab.ipv6, hosts)

	sort.Sort(sortedByID(tab.id))
	sort.Sort(sortedByName(tab.name))
	sort.Sort(sortedByMAC(tab.mac))
	sort.Sort(sortedByIPv4(tab.ipv4))
	sort.Sort(sortedByIPv6(tab.ipv6))

	return tab
}

func (t *Table) Hosts() []*Host {
	return t.name
}

// LookupByNameOrID returns a host for a ID or name
func (t *Table) LookupByNameOrID(id string) *Host {
	h := t.LookupByID(id)
	if h != nil {
		return h
	}
	return t.LookupByName(id)
}

// LookupByID returns a host for a ID address
func (t *Table) LookupByID(id string) *Host {

	host := lookup(t.id, func(h *Host) bool {
		return h.ID >= id
	})

	if host == nil {
		return nil
	}

	if host.ID != id {
		return nil
	}

	return host
}

// LookupByName returns a host for a Name address
func (t *Table) LookupByName(name string) *Host {

	host := lookup(t.name, func(h *Host) bool {
		return h.Name >= name
	})

	if host == nil {
		return nil
	}

	if host.Name != name {
		return nil
	}

	return host
}

// LookupByMAC returns a host for a MAC address
func (t *Table) LookupByMAC(mac net.HardwareAddr) *Host {
	if len(mac) == 0 {
		return nil
	}

	host := lookup(t.mac, func(h *Host) bool {
		return bytes.Compare(h.MAC, mac) >= 0
	})

	if host == nil {
		return nil
	}

	if !bytes.Equal(host.MAC, mac) {
		return nil
	}

	return host
}

// LookupByIPv4 returns a host for a IPv4 address
func (t *Table) LookupByIPv4(ip net.IP) *Host {
	if ip != nil {
		ip = ip.To4()
	}

	if len(ip) == 0 {
		return nil
	}

	host := lookup(t.ipv4, func(h *Host) bool {
		return bytes.Compare(h.IPv4, ip) >= 0
	})

	if host == nil {
		return nil
	}

	if !bytes.Equal(host.IPv4, ip) {
		return nil
	}

	return host
}

// LookupByIPv6 returns a host for a IPv6 address
func (t *Table) LookupByIPv6(ip net.IP) *Host {
	if ip != nil {
		ip = ip.To16()
	}

	if len(ip) == 0 {
		return nil
	}

	host := lookup(t.ipv6, func(h *Host) bool {
		return bytes.Compare(h.IPv6, ip) >= 0
	})

	if host == nil {
		return nil
	}

	if !bytes.Equal(host.IPv6, ip) {
		return nil
	}

	return host
}

func (t *Table) Dump() {
	for _, host := range t.name {
		fmt.Printf("% 10s %v\n", host.Name, host)
	}
}

func lookup(s []*Host, f func(h *Host) bool) *Host {
	length := len(s)

	index := sort.Search(length, func(idx int) bool {
		return f(s[idx])
	})

	if index >= length {
		return nil
	}

	return s[index]
}

type sortedByID []*Host

func (s sortedByID) Len() int           { return len(s) }
func (s sortedByID) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByID) Less(i, j int) bool { return s[i].ID < s[j].ID }

type sortedByName []*Host

func (s sortedByName) Len() int           { return len(s) }
func (s sortedByName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByName) Less(i, j int) bool { return s[i].Name < s[j].Name }

type sortedByMAC []*Host

func (s sortedByMAC) Len() int           { return len(s) }
func (s sortedByMAC) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByMAC) Less(i, j int) bool { return bytes.Compare(s[i].MAC, s[j].MAC) < 0 }

type sortedByIPv4 []*Host

func (s sortedByIPv4) Len() int           { return len(s) }
func (s sortedByIPv4) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByIPv4) Less(i, j int) bool { return bytes.Compare(s[i].IPv4, s[j].IPv4) < 0 }

type sortedByIPv6 []*Host

func (s sortedByIPv6) Len() int           { return len(s) }
func (s sortedByIPv6) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByIPv6) Less(i, j int) bool { return bytes.Compare(s[i].IPv6, s[j].IPv6) < 0 }
