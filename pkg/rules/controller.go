package rules

import (
	"errors"
	"fmt"
	"sync"

	"github.com/fd/switchboard/pkg/ports"
	"github.com/satori/go.uuid"
)

type Controller struct {
	ports *ports.Mapper

	mtx   sync.Mutex
	rules map[string]Rule

	tableMtx sync.RWMutex
	table    *Table
}

func NewController(ports *ports.Mapper) *Controller {
	return &Controller{
		ports: ports,
		rules: make(map[string]Rule),
		table: &Table{},
	}
}

func (c *Controller) GetTable() *Table {
	c.tableMtx.RLock()
	defer c.tableMtx.RUnlock()
	return c.table
}

func (c *Controller) AddRule(rule Rule) (Rule, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	tab := c.GetTable()

	if rule.ID == "" {
		rule.ID = uuid.NewV4().String()
	}
	if !rule.Protocol.Valid() {
		return Rule{}, errors.New("protocol must be set")
	}
	if rule.SrcHostID == "" {
		return Rule{}, errors.New("source host id must be set")
	}
	if rule.SrcPort == 0 {
		return Rule{}, errors.New("source port must be set")
	}
	if rule.DstPort == 0 {
		rule.DstPort = rule.SrcPort
	}

	if r, found := tab.Lookup(rule.Protocol, rule.SrcHostID, rule.SrcPort); found && r.ID != rule.ID {
		return Rule{}, fmt.Errorf("a rule already exists for %s:%s:%d", rule.SrcHostID, rule.Protocol, rule.SrcPort)
	}

	_, err := c.ports.Allocate(rule.SrcHostID, rule.Protocol, rule.SrcPort)
	if err != nil {
		return Rule{}, err
	}

	c.rules[rule.ID] = rule
	c.updateTable()
	return rule, nil
}

func (c *Controller) RemoveRule(id string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	rule, found := c.rules[id]
	if !found {
		return nil
	}

	err := c.ports.Release(rule.SrcHostID, rule.Protocol, rule.SrcPort)
	if err != nil {
		return err
	}

	delete(c.rules, id)
	c.updateTable()
	return nil
}

func (c *Controller) RemoveRulesForHost(hostID string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for id, rule := range c.rules {
		if rule.SrcHostID == hostID {

			err := c.ports.Release(rule.SrcHostID, rule.Protocol, rule.SrcPort)
			if err != nil {
				return err
			}

			delete(c.rules, id)
		}
	}
	c.updateTable()
	return nil
}

func (c *Controller) updateTable() {
	rules := make([]Rule, 0, len(c.rules))
	for _, r := range c.rules {
		rules = append(rules, r)
	}
	tab := buildTable(rules)

	c.tableMtx.Lock()
	c.table = tab
	c.tableMtx.Unlock()
}
