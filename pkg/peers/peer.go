package peers

import (
	"net"
	"time"
)

type Peer struct {
	IP     net.IP
	MAC    net.HardwareAddr
	Expire time.Time
}
