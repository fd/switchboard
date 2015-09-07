package ports

import (
	"errors"
	"math"
	"sort"
	"sync"

	"github.com/fd/switchboard/pkg/protocols"
)

type Mapper struct {
	mtx   sync.RWMutex
	hosts map[string]*host
}

type host struct {
	mtx     sync.Mutex
	NextUDP int
	NextTCP int
	UDP     []int
	TCP     []int
}

func NewMapper() *Mapper {
	return &Mapper{}
}

func (m *Mapper) getHost(hostID string) *host {
	var h *host

	m.mtx.RLock()
	if m.hosts != nil {
		h = m.hosts[hostID]
	}
	m.mtx.RUnlock()

	if h == nil {
		m.mtx.Lock()
		if m.hosts == nil {
			m.hosts = make(map[string]*host, 1024)
		}
		h = m.hosts[hostID]
		if h == nil {
			h = &host{}
			m.hosts[hostID] = h
		}
		m.mtx.Unlock()
	}

	return h
}

func (m *Mapper) Allocate(hostID string, proto protocols.Protocol, port uint16) (uint16, error) {
	h := m.getHost(hostID)

	switch proto {
	case protocols.TCP:
		return h.allocateTCP(port)
	case protocols.UDP:
		return h.allocateUDP(port)
	default:
		return 0, errors.New("unknown protocol")
	}
}

func (m *Mapper) Release(hostID string, proto protocols.Protocol, port uint16) error {
	h := m.getHost(hostID)

	switch proto {
	case protocols.TCP:
		return h.releaseTCP(port)
	case protocols.UDP:
		return h.releaseUDP(port)
	default:
		return errors.New("unknown protocol")
	}
}

func (m *Mapper) ForgetHost(hostID string) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.hosts != nil {
		delete(m.hosts, hostID)
	}
}

func (h *host) allocateTCP(port uint16) (uint16, error) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if len(h.TCP) == (math.MaxUint16 - 1) {
		return 0, errors.New("port pool depleted")
	}

	// Find next unused port
	var wrapCount = 0
	for port == 0 {
		if h.NextTCP >= math.MaxUint16 {
			h.NextTCP = 49152
			wrapCount++
			if wrapCount == 2 {
				return 0, errors.New("port pool depleted")
			}
		} else if h.NextTCP < 49152 {
			h.NextTCP = 49152
		} else {
			h.NextTCP++
		}

		if sort.SearchInts(h.TCP, h.NextTCP) == len(h.TCP) {
			port = uint16(h.NextTCP)
			break
		}
	}

	h.TCP = append(h.TCP, int(port))
	sort.Ints(h.TCP)

	return port, nil
}

func (h *host) allocateUDP(port uint16) (uint16, error) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if len(h.UDP) == (math.MaxUint16 - 1) {
		return 0, errors.New("port pool depleted")
	}

	// Find next unused port
	var wrapCount = 0
	for port == 0 {
		if h.NextUDP >= math.MaxUint16 {
			h.NextUDP = 49152
			wrapCount++
			if wrapCount == 2 {
				return 0, errors.New("port pool depleted")
			}
		} else if h.NextUDP < 49152 {
			h.NextUDP = 49152
		} else {
			h.NextUDP++
		}

		if sort.SearchInts(h.UDP, h.NextUDP) == len(h.UDP) {
			port = uint16(h.NextUDP)
			break
		}
	}

	h.UDP = append(h.UDP, int(port))
	sort.Ints(h.UDP)

	return port, nil
}

func (h *host) releaseTCP(port uint16) error {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	idx := sort.SearchInts(h.TCP, int(port))
	if idx == len(h.TCP) {
		return nil
	}

	copy(h.TCP[idx:], h.TCP[idx+1:])
	h.TCP = h.TCP[:len(h.TCP)-1]
	return nil
}

func (h *host) releaseUDP(port uint16) error {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	idx := sort.SearchInts(h.UDP, int(port))
	if idx == len(h.UDP) {
		return nil
	}

	copy(h.UDP[idx:], h.UDP[idx+1:])
	h.UDP = h.UDP[:len(h.UDP)-1]
	return nil
}
