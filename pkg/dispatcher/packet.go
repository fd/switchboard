package dispatcher

import (
	"net"
	"sync"

	"github.com/fd/switchboard/pkg/hosts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var bufPool sync.Pool
var pktPool sync.Pool

type Packet struct {
	DstHost *hosts.Host
	Flags   uint32

	gopacket.Packet
	layers []gopacket.Layer
	Eth    *layers.Ethernet
	ARP    *layers.ARP
	IPv4   *layers.IPv4
	IPv6   *layers.IPv6
	ICMPv4 *layers.ICMPv4
	ICMPv6 *layers.ICMPv6
	UDP    *layers.UDP
	TCP    *layers.TCP
	buf    []byte
}

func NewPacket(bufSize int) *Packet {
	pkt, _ := pktPool.Get().(*Packet)
	if pkt == nil {
		pkt = &Packet{}
	}

	buf, _ := bufPool.Get().([]byte)
	if buf == nil || len(buf) < bufSize {
		buf = make([]byte, bufSize)
	}

	pkt.buf = buf
	return pkt
}

func (pkt *Packet) Release() {
	if pkt == nil {
		return
	}

	if pkt.buf != nil {
		bufPool.Put(pkt.buf[:cap(pkt.buf)])
		pkt.buf = nil
	}

	*pkt = Packet{}
	pktPool.Put(pkt)
}

func CloneIP(ip net.IP) net.IP {
	var dst net.IP

	if ip4 := ip.To4(); ip4 != nil {
		dst = make(net.IP, 4)
		copy(dst, ip4)
	} else {
		dst = make(net.IP, 16)
		copy(dst, ip.To16())
	}

	return dst
}

func CloneHwAddress(addr net.HardwareAddr) net.HardwareAddr {
	dst := make(net.HardwareAddr, len(addr))
	copy(dst, addr)
	return dst
}
