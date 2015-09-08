package peers

import (
	"bytes"
	"net"
	"sort"
	"sync"
	"time"
)

type Controller struct {
	mtx   sync.RWMutex
	peers []Peer
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) AddPeer(ip net.IP, mac net.HardwareAddr) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	ip = ip.To16()

	idx, found := c.lookup(ip)
	if !found {
		c.peers = append(c.peers, Peer{
			IP:     ip,
			MAC:    mac,
			Expire: time.Now().Add(5 * time.Minute),
		})

		sort.Sort(sortedByIP(c.peers))
	} else {
		c.peers[idx] = Peer{
			IP:     ip,
			MAC:    mac,
			Expire: time.Now().Add(5 * time.Minute),
		}
	}
}

func (c *Controller) Lookup(ip net.IP) net.HardwareAddr {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	idx, found := c.lookup(ip)
	if !found {
		return nil
	}

	peer := c.peers[idx]
	if peer.Expire.Before(time.Now()) {
		return nil
	}

	return peer.MAC
}

func (c *Controller) lookup(ip net.IP) (int, bool) {
	ip = ip.To16()

	idx := sort.Search(len(c.peers), func(idx int) bool {
		return bytes.Compare(c.peers[idx].IP, ip) >= 0
	})

	if idx >= len(c.peers) {
		return 0, false
	}

	peer := c.peers[idx]
	if !bytes.Equal(peer.IP, ip) {
		return 0, false
	}

	return idx, true
}

type sortedByIP []Peer

func (s sortedByIP) Len() int           { return len(s) }
func (s sortedByIP) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortedByIP) Less(i, j int) bool { return bytes.Compare(s[i].IP, s[j].IP) < 0 }
