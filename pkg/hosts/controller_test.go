package hosts

import (
	"fmt"
	"net"
)

func ExampleIPv4List() {
	ctrl := NewController()
	ctrl.AddHost(&Host{IPv4: net.IPv4(172, 18, 0, 3)})
	ctrl.AddHost(&Host{IPv4: net.IPv4(172, 18, 0, 5)})
	ctrl.AddHost(&Host{IPv4: net.IPv4(172, 18, 0, 4)})
	ctrl.AddHost(&Host{})
	ctrl.AddHost(&Host{IPv4: net.IPv4(172, 18, 0, 2)})

	tab := ctrl.GetTable()
	for _, host := range tab.ipv4 {
		fmt.Printf("%s\n", host.IPv4)
	}

	fmt.Println("---")

	if host := tab.LookupByIPv4(net.IPv4(172, 18, 0, 3)); host != nil {
		fmt.Printf("lookup: %s ok\n", host.IPv4)
	} else {
		fmt.Printf("lookup: %s failed\n", net.IPv4(172, 18, 0, 3))
	}

	if host := tab.LookupByIPv4(net.IPv4(172, 18, 0, 5)); host != nil {
		fmt.Printf("lookup: %s ok\n", host.IPv4)
	} else {
		fmt.Printf("lookup: %s failed\n", net.IPv4(172, 18, 0, 5))
	}

	if host := tab.LookupByIPv4(net.IPv4(172, 18, 0, 4)); host != nil {
		fmt.Printf("lookup: %s ok\n", host.IPv4)
	} else {
		fmt.Printf("lookup: %s failed\n", net.IPv4(172, 18, 0, 4))
	}

	if host := tab.LookupByIPv4(net.IPv4(172, 18, 0, 2)); host != nil {
		fmt.Printf("lookup: %s ok\n", host.IPv4)
	} else {
		fmt.Printf("lookup: %s failed\n", net.IPv4(172, 18, 0, 2))
	}

	// Output:
	// <nil>
	// 172.18.0.2
	// 172.18.0.3
	// 172.18.0.4
	// 172.18.0.5
	// ---
	// lookup: 172.18.0.3 ok
	// lookup: 172.18.0.5 ok
	// lookup: 172.18.0.4 ok
	// lookup: 172.18.0.2 ok
}
