package routes

import (
	"fmt"
	"net"

	"github.com/fd/switchboard/pkg/ports"
	"github.com/fd/switchboard/pkg/protocols"
)

func Example() {
	var (
		pm   = ports.NewMapper()
		ctrl = NewController(pm)
	)

	ctrl.AddRoute(&Route{
		Protocol: protocols.TCP,
		HostID:   "host-b",
		Inbound: Stream{
			SrcIP:   net.IPv4(127, 0, 1, 1),
			SrcPort: 22001,
			DstIP:   net.IPv4(127, 0, 1, 2),
			DstPort: 1024,
		},
		Outbound: Stream{
			SrcIP:   net.IPv4(127, 0, 1, 2),
			SrcPort: 22001,
			DstIP:   net.IPv4(127, 0, 1, 3),
			DstPort: 1024,
		},
	})

	ctrl.AddRoute(&Route{
		Protocol: protocols.TCP,
		HostID:   "host-a",
		Inbound: Stream{
			SrcIP:   net.IPv4(127, 0, 0, 1),
			SrcPort: 22001,
			DstIP:   net.IPv4(127, 0, 0, 2),
			DstPort: 1024,
		},
		Outbound: Stream{
			SrcIP:   net.IPv4(127, 0, 0, 2),
			SrcPort: 22001,
			DstIP:   net.IPv4(127, 0, 0, 3),
			DstPort: 1024,
		},
	})

	for _, route := range ctrl.GetTable().routes {
		fmt.Printf("%s\n", route)
	}

	fmt.Println("---")

	tab := ctrl.GetTable()
	fmt.Printf("%s\n", tab.Lookup(protocols.TCP, net.IPv4(127, 0, 1, 1), net.IPv4(127, 0, 1, 2), 22001, 1024))
	fmt.Printf("%s\n", tab.Lookup(protocols.TCP, net.IPv4(127, 0, 1, 3), net.IPv4(127, 0, 1, 2), 1024, 22001))
	fmt.Printf("%s\n", tab.Lookup(protocols.TCP, net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 2), 22001, 1024))
	fmt.Printf("%s\n", tab.Lookup(protocols.TCP, net.IPv4(127, 0, 0, 3), net.IPv4(127, 0, 0, 2), 1024, 22001))

	fmt.Println("---")

	fmt.Printf("%s\n", tab.Lookup(protocols.TCP, net.IPv4(127, 0, 1, 1), net.IPv4(127, 0, 1, 2), 22001, 1025))

	// Output:
	// Route{host-a, TCP, (127.0.0.1:22001 -> 127.0.0.2:1024) => (127.0.0.2:22001 -> 127.0.0.3:1024)}
	// Route{host-a, TCP, (127.0.0.3:1024 -> 127.0.0.2:22001) => (127.0.0.2:1024 -> 127.0.0.1:22001)}
	// Route{host-b, TCP, (127.0.1.1:22001 -> 127.0.1.2:1024) => (127.0.1.2:22001 -> 127.0.1.3:1024)}
	// Route{host-b, TCP, (127.0.1.3:1024 -> 127.0.1.2:22001) => (127.0.1.2:1024 -> 127.0.1.1:22001)}
	// ---
	// Route{host-b, TCP, (127.0.1.1:22001 -> 127.0.1.2:1024) => (127.0.1.2:22001 -> 127.0.1.3:1024)}
	// Route{host-b, TCP, (127.0.1.3:1024 -> 127.0.1.2:22001) => (127.0.1.2:1024 -> 127.0.1.1:22001)}
	// Route{host-a, TCP, (127.0.0.1:22001 -> 127.0.0.2:1024) => (127.0.0.2:22001 -> 127.0.0.3:1024)}
	// Route{host-a, TCP, (127.0.0.3:1024 -> 127.0.0.2:22001) => (127.0.0.2:1024 -> 127.0.0.1:22001)}
	// ---
	// <nil>
}
