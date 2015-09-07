package vmnet

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func TestOpenClose(t *testing.T) {
	wg := &sync.WaitGroup{}

	iface, err := Open("")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("iface=%v\n", iface)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for event := range iface.Events() {
			fmt.Printf("event=%v\n", event)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var buf [2048]byte
		for {
			n, flags, err := iface.ReadPacket(buf[:])
			if err != nil {
				fmt.Printf("read: error=%s\n", err)
				if err == io.EOF {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			p := gopacket.NewPacket(buf[:n], layers.LayerTypeEthernet, gopacket.NoCopy)
			fmt.Printf("read: %08x %s\n", flags, p)
		}
	}()

	time.Sleep(5 * time.Minute)

	err = iface.Close()
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()
}
