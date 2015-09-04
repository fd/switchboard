package hosts

import "net"

type Host struct {
	ID    string
	Name  string
	Local bool

	MAC  net.HardwareAddr
	IPv4 net.IP
	IPv6 net.IP

	Up bool
}

// Clone a host
func (host *Host) Clone() *Host {
	clone := new(Host)
	*clone = *host
	return clone
}
