package workers

import (
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/example/go-service/internal/platform/queue"
	redisplatform "github.com/example/go-service/internal/platform/redis"
)

func NewMux(logger *slog.Logger, redisClient *redisplatform.Client) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeUserWelcome, NewUserWelcomeHandler(logger, redisClient))
	return mux
}
