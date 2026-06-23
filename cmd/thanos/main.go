package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/tinhtran/thanos/internal/cli"
	"github.com/tinhtran/thanos/internal/ui"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx, os.Args[1:], version, os.Stdout, os.Stderr); err != nil {
		ui.Block(os.Stderr, ui.Failure(ui.ErrorDetails{
			Title:  "Thanos Command Failed",
			Reason: err.Error(),
		}))
		os.Exit(1)
	}
}
