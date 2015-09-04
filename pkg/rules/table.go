package rules

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"sort"
)

type Table struct {
	entries []tableEntry
}

type tableEntry struct {
	id uint64
	Rule
}

func buildTable(rules []Rule) *Table {
	var entries = make([]tableEntry, len(rules))
	var sum = fnv.New64a()

	for i, rule := range rules {
		var e tableEntry

		var buf [2]byte
		sum.Reset()
		buf[0] = byte(rule.Protocol)
		sum.Write(buf[:1])
		io.WriteString(sum, rule.SrcHostID)
		binary.BigEndian.PutUint16(buf[:], rule.SrcPort)
		sum.Write(buf[:])
		id := sum.Sum64()

		e.id = id
		e.Rule = rule

		entries[i] = e
	}

	sort.Sort(sortedByID(entries))

	return &Table{entries: entries}
}

type sortedByID []tableEntry

func (s sortedByID) Len() int           { return len(s) }
func (s sortedByID) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByID) Less(i, j int) bool { return s[i].id < s[j].id }

func (tab *Table) Lookup(proto Protocol, hostID string, port uint16) (Rule, bool) {
	var sum = fnv.New64a()
	var buf [2]byte
	sum.Reset()
	buf[0] = byte(proto)
	sum.Write(buf[:1])
	io.WriteString(sum, hostID)
	binary.BigEndian.PutUint16(buf[:], port)
	sum.Write(buf[:])
	id := sum.Sum64()

	nEntries := len(tab.entries)

	idx := sort.Search(nEntries, func(idx int) bool {
		return tab.entries[idx].id >= id
	})

	for _, entry := range tab.entries[idx:] {
		if entry.id != id {
			break
		}

		if entry.Protocol == proto && entry.SrcHostID == hostID && entry.SrcPort == port {
			return entry.Rule, true
		}
	}

	return Rule{}, false
}
