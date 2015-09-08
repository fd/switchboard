package proxy

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/fd/switchboard/pkg/protocols"
	"golang.org/x/net/context"
)

func (p *Proxy) proxyTCP(ctx context.Context) error {
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		return err
	}

	p.TCPPort = uint16(l.Addr().(*net.TCPAddr).Port)

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer l.Close()

		for {
			conn, err := l.AcceptTCP()
			if err != nil {
				log.Printf("PROXY/TCP: error: %s", err)
				time.Sleep(1 * time.Second)
				continue
			}

			go p.proxyTCPStream(ctx, conn)
		}
	}()

	return nil
}

func (p *Proxy) proxyTCPStream(ctx context.Context, src *net.TCPConn) {
	srcRemoteAddr := src.RemoteAddr().(*net.TCPAddr)
	srcLocalAddr := src.LocalAddr().(*net.TCPAddr)

	route := p.routes.GetTable().Lookup(protocols.TCP,
		srcRemoteAddr.IP, srcLocalAddr.IP,
		uint16(srcRemoteAddr.Port), uint16(srcLocalAddr.Port))
	if route == nil {
		src.Close()
		return
	}

	go func() {
		dstAddr := net.TCPAddr{
			IP:   route.Outbound.DstIP,
			Port: int(route.Outbound.DstPort),
		}

		dst, err := net.DialTCP("tcp", nil, &dstAddr)
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
