package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/example/go-service/internal/config"
)

func New(cfg config.LogConfig) *slog.Logger {
	level := new(slog.LevelVar)
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	options := &slog.HandlerOptions{Level: level}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
