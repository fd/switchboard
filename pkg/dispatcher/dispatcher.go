package dispatcher

import (
	"bytes"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fd/switchboard/pkg/hosts"
	"github.com/fd/switchboard/pkg/peers"
	"github.com/fd/switchboard/pkg/ports"
	"github.com/fd/switchboard/pkg/protocols"
	"github.com/fd/switchboard/pkg/proxy"
	"github.com/fd/switchboard/pkg/routes"
	"github.com/fd/switchboard/pkg/rules"
	"github.com/fd/switchboard/pkg/vmnet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/net/context"
)

type VNET struct {
	wg     sync.WaitGroup
	iface  *vmnet.Interface
	ports  *ports.Mapper
	hosts  *hosts.Controller
	rules  *rules.Controller
	routes *routes.Controller
	peers  *peers.Controller
	proxy  *proxy.Proxy
	system *System

	chanEth  chan<- *Packet
	chanArp  chan<- *Packet
	chanIpv4 chan<- *Packet
	chanICMP chan<- *Packet
	chanUDP  chan<- *Packet
	chanTCP  chan<- *Packet
	chanDHCP chan<- *Packet
}

func Run(ctx context.Context) (*VNET, error) {
	rand.Seed(time.Now().Unix())

	p := ports.NewMapper()
	r := routes.NewController(p)

	vnet := &VNET{
		ports:  p,
		routes: r,
		hosts:  hosts.NewController(p),
		rules:  rules.NewController(p),
		peers:  peers.NewController(),
		proxy:  proxy.NewProxy(r),
		system: &System{},
	}

	{ // insert controller
		host, err := vnet.hosts.AddHost(&hosts.Host{
			ID:    "7ce86376-34f0-4951-bead-6152c8291f1c",
			Name:  "controller",
			Local: true,

			IPv6Addrs: []net.IP{net.ParseIP("fd4c:bd56:5cee:8000::2")},

			Up: true,
		})
		if err != nil {
			return nil, err
		}
		log.Printf("insert %s: %v", host.Name, host)
	}

	host, err := vnet.hosts.AddHost(&hosts.Host{
		IPv4Addrs: []net.IP{net.IPv4(172, 18, 0, 2)},
		Local:     true,
		Up:        true,
	})
	if err != nil {
		panic(err)
	}
	log.Printf("insert %s: %v", host.Name, host)

	rule, err := vnet.rules.AddRule(rules.Rule{
		Protocol:  protocols.TCP,
		SrcHostID: host.ID,
		// SrcPort:   80,
		// DstPort:   20559,
		SrcPort: 2376,
		DstIP:   net.IPv4(192, 168, 99, 100),
		// SrcPort: 80,
		// DstIP:   net.IPv4(64, 15, 124, 217),
	})
	if err != nil {
		panic(err)
	}
	log.Printf("insert: %v", rule)

	iface, err := vmnet.Open("31fbf731-e896-4d03-9bc8-7a6221b91860")
	if err != nil {
		return nil, err
	}
	vnet.iface = iface

	vnet.chanEth = vnet.dispatchEthernet(ctx)
	vnet.chanArp = vnet.dispatchARP(ctx)
	vnet.chanIpv4 = vnet.dispatchIPv4(ctx)
	vnet.chanICMP = vnet.dispatchICMP(ctx)
	vnet.chanUDP = vnet.dispatchUDP(ctx)
	vnet.chanTCP = vnet.dispatchTCP(ctx)
	vnet.chanDHCP = vnet.dispatchDHCP(ctx)

	vnet.wg.Add(6)
	go vnet.runReader(ctx)
	go vnet.vmnetCloser(ctx)
	go vnet.gc(ctx)
	go vnet.addGatewayHost(ctx)
	go vnet.addIPv6AddressToVMNET(ctx)
	go vnet.routeIPv4SubnetToController(ctx)

	err = vnet.proxy.Run(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("UUID: %s", vnet.iface.ID())
	log.Printf("MAC:  %s", vnet.iface.HardwareAddr())

	vnet.system.SetControllerMAC(vnet.iface.HardwareAddr())
	return vnet, nil
}

func (vnet *VNET) Wait() {
	vnet.wg.Wait()
}

func (vnet *VNET) System() *System {
	return vnet.system
}

func (vnet *VNET) Hosts() *hosts.Controller {
	return vnet.hosts
}

func (vnet *VNET) Rules() *rules.Controller {
	return vnet.rules
}

func (vnet *VNET) vmnetCloser(ctx context.Context) {
	defer vnet.wg.Done()

	<-ctx.Done()

	err := vnet.iface.Close()
	if err != nil {
		log.Printf("error: %s", err)
	}
}

func (vnet *VNET) gc(ctx context.Context) {
	defer vnet.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			vnet.routes.Expire()
		case <-ctx.Done():
			return
		}
	}
}

func (vnet *VNET) runReader(ctx context.Context) {
	defer vnet.wg.Done()

	var maxPktSize = vnet.iface.MaxPacketSize()

	for {
		var pkt = NewPacket(maxPktSize)

		n, flags, err := vnet.iface.ReadPacket(pkt.buf)
		if err == io.EOF {
			pkt.Release()
			return
		}
		if err != nil {
			pkt.Release()
			log.Printf("error during read: %s", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		pkt.Packet = gopacket.NewPacket(pkt.buf[:n], layers.LayerTypeEthernet, gopacket.NoCopy)
		pkt.Flags = flags
		pkt.layers = pkt.Layers()

		vnet.dispatch(ctx, pkt)
	}
}

func (vnet *VNET) dispatch(ctx context.Context, pkt *Packet) {
	var (
		dst chan<- *Packet
	)

	if len(pkt.layers) == 0 {
		// ignore
		pkt.Release()
		return
	}

	z := pkt.layers[0]
	pkt.layers = pkt.layers[1:]
	switch l := z.(type) {

	case gopacket.ErrorLayer:
		log.Printf("error during read: %s", pkt.ErrorLayer().Error())

	case *layers.Ethernet:
		pkt.Eth = l
		dst = vnet.chanEth

	case *layers.ARP:
		pkt.ARP = l
		dst = vnet.chanArp

	case *layers.IPv4:
		pkt.IPv4 = l
		dst = vnet.chanIpv4
	case *layers.IPv6:
	case *layers.IPv6HopByHop:

	case *layers.ICMPv4:
		pkt.ICMPv4 = l
		dst = vnet.chanICMP
	case *layers.ICMPv6:
		pkt.ICMPv6 = l
		dst = vnet.chanICMP

	case *layers.TCP:
		pkt.TCP = l
		dst = vnet.chanTCP
	case *layers.UDP:
		pkt.UDP = l
		dst = vnet.chanUDP

	}

	if dst == nil {
		// ignore
		pkt.Release()
		return
	}

	select {
	case dst <- pkt:
	case <-ctx.Done():
	}
}

func (vnet *VNET) writePacket(l ...gopacket.SerializableLayer) error {
	buf := getSerializeBuffer()
	defer putSerializeBuffer(buf)

	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	err := gopacket.SerializeLayers(buf, opts, l...)
	if err != nil {
		return err
	}

	// opkt := gopacket.NewPacket(buf.Bytes(), layers.LayerTypeEthernet, gopacket.NoCopy)
	// log.Printf("WRITE: %08x %s\n", 0, opkt.Dump())

	_, err = vnet.iface.WritePacket(buf.Bytes(), 0)
	if err != nil {
		return err
	}

	return nil
}

func (vnet *VNET) addGatewayHost(ctx context.Context) {
	defer vnet.wg.Done()

	vnet.system.WaitForGatewayIPv4()

	host, err := vnet.hosts.AddHost(&hosts.Host{
		ID:    "d9c62f0c-7936-4384-8d85-4587561a7142",
		Name:  "gateway",
		Local: true,

		IPv4Addrs: []net.IP{vnet.system.GatewayIPv4()},
		IPv6Addrs: []net.IP{net.ParseIP("fd4c:bd56:5cee:8000::1")},

		Up: true,
	})
	if err != nil {
		panic(err)
	}
	log.Printf("insert gateway: %v", host)
}

func (vnet *VNET) addIPv6AddressToVMNET(ctx context.Context) {
	defer vnet.wg.Done()

	vnet.system.WaitForGatewayMAC()

	var (
		iface net.Interface
		found bool
	)

	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, i := range ifaces {
		if bytes.Equal(i.HardwareAddr, vnet.system.GatewayMAC()) {
			// found iface
			iface = i
			found = true
			break
		}
	}
	if !found {
		panic(errors.New("unable to find interface"))
	}

	err = exec.Command("sudo", "ifconfig", iface.Name, "inet6", "fd4c:bd56:5cee:8000::1", "prefixlen", "48").Run()
	if err != nil {
		panic(err)
	}
}

func (vnet *VNET) routeIPv4SubnetToController(ctx context.Context) {
	defer vnet.wg.Done()

	vnet.system.WaitForControllerIPv4()
	vnet.system.WaitForGatewayMAC()

	vnet.hosts.HostAddIPv4("controller", net.IPv4(172, 18, 0, 1))

	// sudo route -n add -net 172.18.0.0/16 192.168.128.7
	log.Printf("exec: %v", []string{"route", "-n", "add", "-net", "172.18.0.0/16", vnet.system.ControllerIPv4().String()})
	err := exec.Command("sudo", "route", "-n", "add", "-net", "172.18.0.0/16", vnet.system.ControllerIPv4().String()).Run()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}

	var (
		ifaceName   string
		coIfaceName string
	)
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}
	for _, iface := range ifaces {
		if !bytes.Equal(iface.HardwareAddr, vnet.system.GatewayMAC()) {
			continue
		}
		output, err := exec.Command("ifconfig", iface.Name).Output()
		if err != nil {
			log.Printf("ROUTE/error: %s", err)
			return
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "member:") {
				continue
			}
			line = strings.TrimPrefix(line, "member:")
			line = strings.TrimSpace(line)
			if idx := strings.IndexByte(line, ' '); idx > 0 {
				line = line[:idx]
			}
			ifaceName = iface.Name
			coIfaceName = line
			break
		}
		break
	}

	// sudo ifconfig bridge100 -hostfilter en4
	log.Printf("exec: %v", []string{"ifconfig", ifaceName, "-hostfilter", coIfaceName})
	err = exec.Command("sudo", "ifconfig", ifaceName, "-hostfilter", coIfaceName).Run()
	if err != nil {
		log.Printf("ROUTE/error: %s", err)
		return
	}
}

// host:  127.0.0.1:5000    -> 192.168.64.25:6000
// guest: 192.168.64.1:5000 -> 192.168.64.25:6000
//
// forward: (192.168.64.25:6000 -> 127.0.0.1:7000)
// guest: 192.168.64.2:5000 -> 192.168.64.1:7000
// host:  192.168.64.2:5000 -> 127.0.0.1:7000
//
//
// sudo ifconfig vmnet1 inet6 fd4c:bd56:5cee::1 prefixlen 48
// net        = fd4c:bd56:5cee::/48
// gateway    = fd4c:bd56:5cee:8000::1
// local      = fd4c:bd56:5cee:8000::/64
// remote-net = fd4c:bd56:5cee:0000::/49
// remote     = fd4c:bd56:5cee:0000::/64
//
// this allows for:
// - 1 gateway
// - 1 local host
// - 2^15 remote hosts
// - 2^64-2 guests per host
