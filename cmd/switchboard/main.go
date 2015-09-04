package main

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/context"

	"github.com/fd/switchboard/pkg/dispatcher"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go quitSignal(cancel)

	vnet, err := dispatcher.Run(ctx)
	assert(err)

	defer vnet.Wait()
}

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

func quitSignal(cancel func()) {
	defer cancel()

	c := make(chan os.Signal)
	defer signal.Stop(c)
	go signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
