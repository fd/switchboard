package dispatcher

import (
	"net"
	"sync"
	"time"
)

type System struct {
	mtx sync.RWMutex
	cnd *sync.Cond

	gatewayMAC              net.HardwareAddr
	gatewayIPv4             net.IP
	controllerMAC           net.HardwareAddr
	controllerIPv4          net.IP
	controllerLastDHCPRenew time.Time
}

// WaitForGatewayMAC waits until the gateway MAC addresses is known
func (sys *System) WaitForGatewayMAC() {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()

	if sys.cnd == nil {
		sys.ensureCondExistsInRLocker()
	}

	for sys.gatewayMAC == nil {
		sys.cnd.Wait()
	}
}

// GatewayMAC returns the MAC addresses of the gateway
func (sys *System) GatewayMAC() net.HardwareAddr {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()
	return sys.gatewayMAC
}

// SetGatewayMAC sets the MAC addresses of the gateway
func (sys *System) SetGatewayMAC(addr net.HardwareAddr) {
	sys.mtx.Lock()
	defer sys.mtx.Unlock()

	if sys.cnd == nil {
		sys.cnd = sync.NewCond(sys.mtx.RLocker())
	}

	sys.gatewayMAC = CloneHwAddress(addr)
	sys.cnd.Broadcast()
}

// WaitForGatewayIPv4 waits until the gateway IPv4 addresses is known
func (sys *System) WaitForGatewayIPv4() {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()

	if sys.cnd == nil {
		sys.ensureCondExistsInRLocker()
	}

	for sys.gatewayIPv4 == nil {
		sys.cnd.Wait()
	}
}

// GatewayIPv4 returns the IPv4 addresses of the gateway
func (sys *System) GatewayIPv4() net.IP {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()
	return sys.gatewayIPv4
}

// SetGatewayIPv4 sets the IPv4 addresses of the gateway
func (sys *System) SetGatewayIPv4(addr net.IP) {
	sys.mtx.Lock()
	defer sys.mtx.Unlock()

	if sys.cnd == nil {
		sys.cnd = sync.NewCond(sys.mtx.RLocker())
	}

	sys.gatewayIPv4 = CloneIP(addr).To4()
	sys.cnd.Broadcast()
}

// WaitForControllerMAC waits until the controller MAC addresses is known
func (sys *System) WaitForControllerMAC() {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()

	if sys.cnd == nil {
		sys.ensureCondExistsInRLocker()
	}

	for sys.controllerMAC == nil {
		sys.cnd.Wait()
	}
}

// ControllerMAC returns the MAC addresses of the controller
func (sys *System) ControllerMAC() net.HardwareAddr {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()
	return sys.controllerMAC
}

// SetControllerMAC sets the MAC addresses of the controller
func (sys *System) SetControllerMAC(addr net.HardwareAddr) {
	sys.mtx.Lock()
	defer sys.mtx.Unlock()

	if sys.cnd == nil {
		sys.cnd = sync.NewCond(sys.mtx.RLocker())
	}

	sys.controllerMAC = CloneHwAddress(addr)
	sys.cnd.Broadcast()
}

// WaitForControllerIPv4 waits until the controller IPv4 addresses is known
func (sys *System) WaitForControllerIPv4() {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()

	if sys.cnd == nil {
		sys.ensureCondExistsInRLocker()
	}

	for sys.controllerIPv4 == nil {
		sys.cnd.Wait()
	}
}

// ControllerIPv4 returns the IPv4 addresses of the controller
func (sys *System) ControllerIPv4() net.IP {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()
	return sys.controllerIPv4
}

// SetControllerIPv4 sets the IPv4 addresses of the controller
func (sys *System) SetControllerIPv4(addr net.IP) {
	sys.mtx.Lock()
	defer sys.mtx.Unlock()

	if sys.cnd == nil {
		sys.cnd = sync.NewCond(sys.mtx.RLocker())
	}

	sys.controllerIPv4 = CloneIP(addr).To4()
	sys.controllerLastDHCPRenew = time.Now()
	sys.cnd.Broadcast()
}

// ControllerLastDHCPRenew returns the last time a DHCP negotiation was performed
func (sys *System) ControllerLastDHCPRenew() time.Time {
	sys.mtx.RLock()
	defer sys.mtx.RUnlock()
	return sys.controllerLastDHCPRenew
}

func (sys *System) ensureCondExistsInRLocker() {
	sys.mtx.RUnlock()
	sys.mtx.Lock()

	if sys.cnd == nil {
		sys.cnd = sync.NewCond(sys.mtx.RLocker())
	}

	sys.mtx.Unlock()
	sys.mtx.RLock()
}
