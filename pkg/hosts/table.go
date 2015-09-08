package hosts

import (
	"bytes"
	"fmt"
	"net"
	"sort"
	"strings"
)

type Table struct {
	id   []*Host
	name []*Host
	ipv4 []ipEntry
	ipv6 []ipEntry
}

type ipEntry struct {
	ip   net.IP
	host *Host
}

func buildTable(hosts []*Host) *Table {
	tab := &Table{
		id:   make([]*Host, len(hosts)),
		name: make([]*Host, len(hosts)),
		ipv4: make([]ipEntry, 0, len(hosts)*2),
		ipv6: make([]ipEntry, 0, len(hosts)*2),
	}

	copy(tab.id, hosts)
	copy(tab.name, hosts)

	for _, host := range hosts {
		for _, ip := range host.IPv4Addrs {
			tab.ipv4 = append(tab.ipv4, ipEntry{ip, host})
		}
		for _, ip := range host.IPv6Addrs {
			tab.ipv6 = append(tab.ipv6, ipEntry{ip, host})
		}
	}

	sort.Sort(sortedByID(tab.id))
	sort.Sort(sortedByName(tab.name))
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

	// Allow short ID
	if len(id) >= 8 && strings.HasPrefix(host.ID, id) {
		id = host.ID
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

// LookupByIPv4 returns a host for a IPv4 address
func (t *Table) LookupByIPv4(ip net.IP) *Host {
	if ip != nil {
		ip = ip.To4()
	}

	if len(ip) == 0 {
		return nil
	}

	entry := lookupIP(t.ipv4, func(e ipEntry) bool {
		return bytes.Compare(e.ip, ip) >= 0
	})

	if entry.host == nil {
		return nil
	}

	if !bytes.Equal(entry.ip, ip) {
		return nil
	}

	return entry.host
}

// LookupByIPv6 returns a host for a IPv6 address
func (t *Table) LookupByIPv6(ip net.IP) *Host {
	if ip != nil {
		ip = ip.To16()
	}

	if len(ip) == 0 {
		return nil
	}

	entry := lookupIP(t.ipv6, func(e ipEntry) bool {
		return bytes.Compare(e.ip, ip) >= 0
	})

	if entry.host == nil {
		return nil
	}

	if !bytes.Equal(entry.ip, ip) {
		return nil
	}

	return entry.host
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

func lookupIP(s []ipEntry, f func(h ipEntry) bool) ipEntry {
	length := len(s)

	index := sort.Search(length, func(idx int) bool {
		return f(s[idx])
	})

	if index >= length {
		return ipEntry{}
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

type sortedByIPv4 []ipEntry

func (s sortedByIPv4) Len() int           { return len(s) }
func (s sortedByIPv4) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByIPv4) Less(i, j int) bool { return bytes.Compare(s[i].ip, s[j].ip) < 0 }

type sortedByIPv6 []ipEntry

func (s sortedByIPv6) Len() int           { return len(s) }
func (s sortedByIPv6) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByIPv6) Less(i, j int) bool { return bytes.Compare(s[i].ip, s[j].ip) < 0 }
