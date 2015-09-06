package rules

import (
	"net"

	"github.com/fd/switchboard/pkg/protocols"
)

type Rule struct {
	ID       string
	Protocol protocols.Protocol

	SrcHostID string
	SrcPort   uint16

	DstIP   net.IP
	DstPort uint16
}
