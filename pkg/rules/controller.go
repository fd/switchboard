package rules

import (
	"errors"
	"fmt"
	"sync"

	"github.com/satori/go.uuid"
)

type Controller struct {
	mtx   sync.Mutex
	rules map[string]Rule

	tableMtx sync.RWMutex
	table    *Table
}

func NewController() *Controller {
	return &Controller{
		rules: make(map[string]Rule),
		table: &Table{},
	}
}

func (c *Controller) GetTable() *Table {
	c.tableMtx.RLock()
	defer c.tableMtx.RUnlock()
	return c.table
}

func (c *Controller) AddRule(rule Rule) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	tab := c.GetTable()

	if rule.ID == "" {
		rule.ID = uuid.NewV4().String()
	}
	if rule.Protocol == Invalid {
		return errors.New("protocol must be set")
	}
	if rule.SrcHostID == "" {
		return errors.New("source host id must be set")
	}
	if rule.SrcPort == 0 {
		return errors.New("source port must be set")
	}
	if rule.DstPort == 0 {
		rule.DstPort = rule.SrcPort
	}

	if r, found := tab.Lookup(rule.Protocol, rule.SrcHostID, rule.SrcPort); found && r.ID != rule.ID {
		return fmt.Errorf("a rule already exists for %s:%s:%d", rule.SrcHostID, rule.Protocol, rule.SrcPort)
	}

	c.rules[rule.ID] = rule
	c.updateTable()
	return nil
}

func (c *Controller) RemoveRule(id string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	delete(c.rules, id)
	c.updateTable()
}

func (c *Controller) RemoveRulesForHost(hostID string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for id, rule := range c.rules {
		if rule.SrcHostID == hostID {
			delete(c.rules, id)
		}
	}
	c.updateTable()
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
