package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/queue"
)

const webhookDeliveryMaxRetry = 5

// Dispatcher fans a job's terminal-state event out to the owner's registered
// webhooks. Each delivery is enqueued as its own retryable asynq task rather
// than delivered inline, so a slow or failing subscriber never blocks job
// processing and gets independent exponential-backoff retries.
type Dispatcher struct {
	webhookRepo repository.WebhookRepository
	queueClient *queue.Client
}

func NewDispatcher(webhookRepo repository.WebhookRepository, queueClient *queue.Client) *Dispatcher {
	return &Dispatcher{
		webhookRepo: webhookRepo,
		queueClient: queueClient,
	}
}

// DispatchJobEvent enqueues a webhook-delivery task for every webhook the job's
// owner has registered. It is best-effort and never returns an error: a failure
// to notify subscribers must not fail the job itself.
func (d *Dispatcher) DispatchJobEvent(ctx context.Context, job *entity.Job) {
	if job.UserID == nil {
		return // job has no owner — nothing to notify
	}

	log := logger.FromContext(ctx)

	webhooks, err := d.webhookRepo.ListByUserID(ctx, *job.UserID)
	if err != nil {
		log.Error("dispatch: listing webhooks failed", "user_id", job.UserID.String(), "error", err.Error())
		return
	}
	if len(webhooks) == 0 {
		return
	}

	event := entity.WebhookDeliveryEvent{
		JobID:     job.ID.String(),
		Type:      string(job.Type),
		Status:    string(job.Status),
		Timestamp: job.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}

	for _, wh := range webhooks {
		payload := queue.WebhookDeliveryPayload{
			WebhookID: wh.ID.String(),
			Event:     event,
		}
		if err := d.queueClient.Enqueue(queue.TypeWebhookDelivery, payload, queue.EnqueueOptions{
			MaxRetry: webhookDeliveryMaxRetry,
		}); err != nil {
			log.Error("dispatch: enqueuing delivery failed",
				"webhook_id", wh.ID.String(),
				"error", err.Error(),
			)
			continue
		}
		log.Info("dispatch: delivery enqueued",
			"webhook_id", wh.ID.String(),
			"status", event.Status,
		)
	}
}

// GenerateSecret creates a cryptographically random webhook secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
