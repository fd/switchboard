package rules

import "net"

type Protocol uint8

const (
	Invalid Protocol = iota
	TCP
	UDP
)

type Rule struct {
	ID       string
	Protocol Protocol

	SrcHostID string
	SrcPort   uint16

	DstIP   net.IP
	DstPort uint16
}
