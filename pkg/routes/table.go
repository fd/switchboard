package routes

import (
	"net"
	"sort"

	"github.com/fd/switchboard/pkg/protocols"
)

type Table struct {
	routes []*Route
}

func buildTable(routes []*Route) *Table {
	tab := &Table{
		routes: make([]*Route, len(routes)),
	}

	copy(tab.routes, routes)

	sort.Sort(sortedByInbound(tab.routes))

	return tab
}

func (tab *Table) Lookup(
	proto protocols.Protocol,
	srcIP, dstIP net.IP,
	srcPort, dstPort uint16,
) *Route {
	target := Route{}
	target.Protocol = proto
	target.SetInboundSource(srcIP.To16(), srcPort)
	target.SetInboundDestination(dstIP.To16(), dstPort)

	sze := len(tab.routes)
	idx := sort.Search(sze, func(idx int) bool {
		return !lessInbound(tab.routes[idx], &target)
	})

	if idx >= sze {
		return nil
	}

	if lessInbound(&target, tab.routes[idx]) {
		return nil
	}

	return tab.routes[idx]
}

type sortedByInbound []*Route

func (s sortedByInbound) Len() int           { return len(s) }
func (s sortedByInbound) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByInbound) Less(i, j int) bool { return lessInbound(s[i], s[j]) }
