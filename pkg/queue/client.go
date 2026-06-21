package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
)

const (
	TypeEmail   = "job:email"
	TypeSMS     = "job:sms"
	TypeWebhook = "job:webhook"
	TypeReport  = "job:report"

	// TypeWebhookDelivery is an internal task: deliver a job's terminal-state
	// event to one registered webhook. Each delivery is retried independently.
	TypeWebhookDelivery = "job:webhook-delivery"
)

// DefaultQueue is the asynq queue name all tasks are enqueued to. Producers and
// consumers (inspector, task deletion) must agree on this value.
const DefaultQueue = "default"

// TaskTypeForJob maps a domain job type to its asynq task type. Kept here so the
// producer (job use case) and admin retry path share one mapping.
func TaskTypeForJob(t entity.JobType) string {
	switch t {
	case entity.JobTypeEmail:
		return TypeEmail
	case entity.JobTypeSMS:
		return TypeSMS
	case entity.JobTypeWebhook:
		return TypeWebhook
	case entity.JobTypeReport:
		return TypeReport
	default:
		return string(t)
	}
}

// WebhookDeliveryPayload is the task payload for TypeWebhookDelivery. The
// secret is never placed on the queue — the delivery handler loads it from the
// webhook record by ID at send time.
type WebhookDeliveryPayload struct {
	WebhookID string                      `json:"webhook_id"`
	Event     entity.WebhookDeliveryEvent `json:"event"`
}

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
