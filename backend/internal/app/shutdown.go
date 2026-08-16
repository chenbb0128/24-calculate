package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ShutdownHTTP gracefully stops accepting new requests and waits for active
// requests to finish within the configured timeout.
func ShutdownHTTP(parent context.Context, server *http.Server, logger *slog.Logger, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger.InfoContext(parent, "shutting down api server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown api server: %w", err)
	}

	logger.Info("api server stopped")
	return nil
}
