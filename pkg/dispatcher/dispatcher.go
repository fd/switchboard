package dispatcher

import (
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/fd/switchboard/pkg/hosts"
	"github.com/fd/switchboard/pkg/ports"
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

	vnet := &VNET{
		ports:  p,
		hosts:  hosts.NewController(p),
		rules:  rules.NewController(p),
		routes: routes.NewController(p),
	}

	host, err := vnet.hosts.AddHost(&hosts.Host{
		IPv4:  net.IPv4(172, 18, 0, 2),
		Local: true,
		Up:    true,
	})
	if err != nil {
		panic(err)
	}
	log.Printf("insert %s: %v", host.Name, host)

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

	vnet.wg.Add(2)
	go vnet.runReader(ctx)
	go vnet.vmnetCloser(ctx)

	log.Printf("UUID: %s", vnet.iface.ID())
	log.Printf("MAC:  %s", vnet.iface.HardwareAddr())

	return vnet, nil
}

func (vnet *VNET) Wait() {
	vnet.wg.Wait()
}

func (vnet *VNET) Controller() *hosts.Controller {
	return vnet.hosts
}

func (vnet *VNET) vmnetCloser(ctx context.Context) {
	defer vnet.wg.Done()

	<-ctx.Done()

	err := vnet.iface.Close()
	if err != nil {
		log.Printf("error: %s", err)
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
