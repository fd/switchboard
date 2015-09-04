package dispatcher

import "golang.org/x/net/context"

func (vnet *VNET) dispatchUDP(ctx context.Context) chan<- *Packet {
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

			// DHCP
			if pkt.UDP.DstPort == 68 {
				select {
				case vnet.chanDHCP <- pkt:
				case <-ctx.Done():
					pkt.Release()
					return
				}
				continue
			}

			// fmt.Printf("UDPv4: %08x %s\n", pkt.Flags, pkt.String())
		}
	}()

	return in
}
