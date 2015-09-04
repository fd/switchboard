package dispatcher

import "golang.org/x/net/context"

func (vnet *VNET) dispatchEthernet(ctx context.Context) chan<- *Packet {
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

			vnet.dispatch(ctx, pkt)
		}
	}()

	return in
}
