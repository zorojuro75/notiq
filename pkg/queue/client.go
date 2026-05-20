package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypeEmail   = "job:email"
	TypeSMS     = "job:sms"
	TypeWebhook = "job:webhook"
	TypeReport  = "job:report"
)

type Client struct {
	asynq *asynq.Client
}

func NewClient(redisAddr, redisPassword string, redisDB int) *Client {
	c := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	return &Client{asynq: c}
}

type EnqueueOptions struct {
	MaxRetry    int
	ScheduledAt *time.Time
	TaskID      string
}

func (c *Client) Enqueue(taskType string, payload any, opts EnqueueOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	task := asynq.NewTask(taskType, data)

	asynqOpts := []asynq.Option{
		asynq.MaxRetry(opts.MaxRetry),
	}

	if opts.TaskID != "" {
		asynqOpts = append(asynqOpts, asynq.TaskID(opts.TaskID))
	}

	if opts.ScheduledAt != nil {
		asynqOpts = append(asynqOpts, asynq.ProcessAt(*opts.ScheduledAt))
	}

	_, err = c.asynq.Enqueue(task, asynqOpts...)
	return err
}

func (c *Client) Close() error {
	return c.asynq.Close()
}
