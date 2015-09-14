package main

import (
	"log"

	"golang.org/x/net/context"

	"github.com/fd/switchboard/pkg/plugin"
)

func main() { plugin.Run(handler) }

func handler(ctx context.Context, plugin *plugin.Plugin) error {
	config, err := loadPowConfig()
	if err != nil {
		return err
	}

	log.Print(config)

	<-ctx.Done()
	return nil
}
