package hosts

import "net"

type Host struct {
	ID    string
	Name  string
	Local bool

	MAC       net.HardwareAddr
	IPv4Addrs []net.IP
	IPv6Addrs []net.IP

	Up bool
}

// Clone a host
func (host *Host) Clone() *Host {
	clone := new(Host)
	*clone = *host
	return clone
}
