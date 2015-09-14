package server

import (
	"log"
	"net"

	"github.com/fd/switchboard/pkg/api/protocol"
	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/rules"
	"golang.org/x/net/context"
)

var _ protocol.RulesServer = (*rulesServer)(nil)

type rulesServer struct {
	rules *rules.Controller
}

func (s *rulesServer) Add(ctx context.Context, req *protocol.RuleAddReq) (*protocol.RuleAddRes, error) {
	rule := rules.Rule{}
	rule.Protocol = protocols.Protocol(req.Protocol)
	rule.SrcHostID = req.SrcHostId
	rule.SrcPort = uint16(req.SrcPort)
	rule.DstIP = net.ParseIP(req.DstIp)
	rule.DstPort = uint16(req.DstPort)

	rule, err := s.rules.AddRule(rule)
	if err != nil {
		return nil, err
	}

	log.Printf("rule=%v", rule)

	return &protocol.RuleAddRes{}, nil
}

func (s *rulesServer) Clear(ctx context.Context, req *protocol.RuleClearReq) (*protocol.RuleClearRes, error) {
	err := s.rules.RemoveRulesForHost(req.HostId)
	if err != nil {
		return nil, err
	}

	return &protocol.RuleClearRes{}, nil
}
