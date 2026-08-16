package app

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/example/go-service/internal/config"
	appLogger "github.com/example/go-service/internal/platform/logger"
	queueplatform "github.com/example/go-service/internal/platform/queue"
	redisplatform "github.com/example/go-service/internal/platform/redis"
	"github.com/example/go-service/internal/workers"
)

func RunWorker(ctx context.Context, cfg *config.Config) error {
	logger := appLogger.New(cfg.Log)
	redisClient := redisplatform.New(cfg.Redis)
	if err := redisClient.Check(ctx); err != nil {
		_ = redisClient.Close()
		return fmt.Errorf("check redis: %w", err)
	}
	defer redisClient.Close()

	server := queueplatform.NewServer(cfg.Queue, cfg.Redis)
	mux := workers.NewMux(logger, redisClient)
	if err := server.Start(mux); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	logger.Info("worker started", "queue", cfg.Queue.Name, "concurrency", cfg.Queue.Concurrency)

	<-ctx.Done()
	logger.Info("shutting down worker")
	server.Shutdown()
	logger.Info("worker stopped")
	return nil
}

var _ asynq.Handler = (*workers.UserWelcomeHandler)(nil)
