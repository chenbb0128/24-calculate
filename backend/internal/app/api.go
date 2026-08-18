package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/example/go-service/internal/config"
)

// RunAPI starts the HTTP server and waits until the process context is
// cancelled or the server exits unexpectedly.
func RunAPI(ctx context.Context, cfg *config.Config) (runErr error) {
	runtime, err := BootstrapAPI(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := runtime.Close(); err != nil && runErr == nil {
			runErr = fmt.Errorf("close api resources: %w", err)
		}
	}()

	server := runtime.Server
	logger := runtime.Logger
	if runtime.Player != nil {
		runtime.Player.StartFriendBotBackground(ctx)
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api server started", "address", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve api: %w", err)
	case <-ctx.Done():
		return ShutdownHTTP(ctx, server, logger, cfg.Server.ShutdownTimeout)
	}
}
