package proxy

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/fd/switchboard/pkg/protocols"
	"golang.org/x/net/context"
)

func (p *Proxy) proxyUDP(ctx context.Context) error {
	l, err := net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	go func() {
		defer l.Close()

		for {
			conn, err := l.AcceptUDP()
			if err != nil {
				log.Printf("PROXY/UDP: error: %s", err)
				time.Sleep(1 * time.Second)
				continue
			}

			go p.proxyUDPStream(ctx, conn)
		}
	}()

	return nil
}

func (p *Proxy) proxyUDPStream(ctx context.Context, src *net.UDPConn) {
	srcRemoteAddr := src.RemoteAddr().(*net.UDPAddr)
	srcLocalAddr := src.LocalAddr().(*net.UDPAddr)

	route := p.routes.GetTable().Lookup(protocols.UDP,
		srcRemoteAddr.IP, srcLocalAddr.IP,
		uint16(srcRemoteAddr.Port), uint16(srcLocalAddr.Port))
	if route == nil {
		src.Close()
		return
	}

	go func() {
		dstAddr := net.UDPAddr{
			IP:   route.Outbound.DstIP,
			Port: int(route.Outbound.DstPort),
		}

		dst, err := net.DialUDP("udp", nil, &dstAddr)
		if err != nil {
			src.Close()
			return
		}

		dst.SetKeepAlivePeriod(10 * time.Second)
		src.SetKeepAlivePeriod(10 * time.Second)

		go func() {
			<-ctx.Done()
			src.Close()
			dst.Close()
		}()

		go func() {
			defer dst.CloseWrite()
			defer src.CloseRead()
			io.Copy(dst, src)
		}()

		go func() {
			defer src.CloseWrite()
			defer dst.CloseRead()
			io.Copy(src, dst)
		}()
	}()
}
