package hosts

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
)

func generateIPv6(local bool) (net.IP, error) {
	addr := make(net.IP, 16)
	_, err := io.ReadFull(rand.Reader, addr[:])
	if err != nil {
		return nil, fmt.Errorf("failed to generate IPv6: %s", err)
	}

	addr[0] = 0xfd
	addr[1] = 0x4c
	addr[2] = 0xbd
	addr[3] = 0x56
	addr[4] = 0x5c
	addr[5] = 0xee

	if local {
		addr[6] = 0x80
		addr[7] = 0x00
	} else {
		addr[6] = 0x00
		addr[7] = 0x00
	}

	return addr, nil
}
