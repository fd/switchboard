package main

import (
	"golang.org/x/net/context"

	"github.com/fd/switchboard/pkg/plugin"
)

func main() { plugin.Run(handler) }

func handler(ctx context.Context, plugin *plugin.Plugin) error {
	return watch(ctx, plugin)
}
