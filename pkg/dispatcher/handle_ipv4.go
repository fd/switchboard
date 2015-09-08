package dispatcher

import (
	"bytes"

	"golang.org/x/net/context"
)

func (vnet *VNET) dispatchIPv4(ctx context.Context) chan<- *Packet {
	var in = make(chan *Packet)

	vnet.wg.Add(1)
	go func() {
		defer vnet.wg.Done()

		for {
			var pkt *Packet

			select {
			case pkt = <-in:
			case <-ctx.Done():
				return
			}

			host := vnet.hosts.GetTable().LookupByIPv4(pkt.IPv4.DstIP)
			if host == nil {
				if bytes.Equal(pkt.IPv4.DstIP, vnet.system.ControllerIPv4()) {
					pkt.DstHost = vnet.hosts.GetTable().LookupByName("controller")
				}
			} else {
				pkt.DstHost = host
			}

			// fmt.Printf("IPv4: %08x %s\n", pkt.Flags, pkt.String())
			vnet.dispatch(ctx, pkt)
		}
	}()

	return in
}
