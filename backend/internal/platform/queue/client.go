package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/example/go-service/internal/config"
)

type Client struct {
	client      *asynq.Client
	queueName   string
	taskTimeout time.Duration
	maxRetry    int
}

func NewClient(queueCfg config.QueueConfig, redisCfg config.RedisConfig) *Client {
	return &Client{
		client: asynq.NewClient(asynq.RedisClientOpt{
			Addr:     redisCfg.Addr,
			Password: redisCfg.Password,
			DB:       redisCfg.DB,
		}),
		queueName:   queueCfg.Name,
		taskTimeout: queueCfg.TaskTimeout,
		maxRetry:    queueCfg.MaxRetry,
	}
}

func (c *Client) EnqueueUserWelcome(ctx context.Context, userID uint64) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("queue client is not initialized")
	}
	task, err := NewUserWelcomeTask(userID)
	if err != nil {
		return err
	}
	_, err = c.client.EnqueueContext(ctx, task,
		asynq.Queue(c.queueName),
		asynq.Timeout(c.taskTimeout),
		asynq.MaxRetry(c.maxRetry),
		asynq.TaskID(fmt.Sprintf("user-welcome:%d", userID)),
	)
	return err
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
