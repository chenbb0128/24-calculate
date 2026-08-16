package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/example/go-service/internal/platform/queue"
	redisplatform "github.com/example/go-service/internal/platform/redis"
)

type UserWelcomeHandler struct {
	logger *slog.Logger
	redis  *redisplatform.Client
}

func NewUserWelcomeHandler(logger *slog.Logger, redisClient *redisplatform.Client) *UserWelcomeHandler {
	return &UserWelcomeHandler{logger: logger, redis: redisClient}
}

func (h *UserWelcomeHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.UserWelcomePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode user welcome payload: %w", err)
	}
	if payload.Version != 1 || payload.UserID == 0 {
		return fmt.Errorf("invalid user welcome payload")
	}

	if h.redis != nil {
		created, err := h.redis.SetNX(ctx, redisplatform.WelcomeTaskKey(payload.UserID), "done", 7*24*time.Hour).Result()
		if err != nil {
			return fmt.Errorf("set user welcome idempotency key: %w", err)
		}
		if !created {
			return nil
		}
	}

	h.logger.InfoContext(ctx, "user welcome task processed", "user_id", payload.UserID)
	return nil
}
