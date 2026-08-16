package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/go-service/internal/app"
	"github.com/example/go-service/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.RunWorker(ctx, cfg)
}
