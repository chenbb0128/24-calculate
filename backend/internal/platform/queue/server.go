package queue

import (
	"github.com/hibiken/asynq"

	"github.com/example/go-service/internal/config"
)

func NewServer(queueCfg config.QueueConfig, redisCfg config.RedisConfig) *asynq.Server {
	return asynq.NewServer(asynq.RedisClientOpt{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	}, asynq.Config{
		Concurrency: queueCfg.Concurrency,
		Queues: map[string]int{
			queueCfg.Name: 1,
		},
	})
}
