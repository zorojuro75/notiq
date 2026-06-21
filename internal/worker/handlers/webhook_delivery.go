package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/queue"
	"github.com/zorojuro75/notiq/pkg/safehttp"
	"github.com/zorojuro75/notiq/pkg/signature"
)

// WebhookDeliveryHandler delivers a single job event to one registered webhook.
// Returning an error makes asynq retry the delivery with backoff.
type WebhookDeliveryHandler struct {
	webhookRepo repository.WebhookRepository
	httpClient  *http.Client
}

func NewWebhookDeliveryHandler(webhookRepo repository.WebhookRepository, allowPrivateTargets bool) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{
		webhookRepo: webhookRepo,
		httpClient:  safehttp.NewClient(10*time.Second, allowPrivateTargets),
	}
}

func (h *WebhookDeliveryHandler) Handle(ctx context.Context, task *asynq.Task) error {
	var p queue.WebhookDeliveryPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		// malformed payload will never succeed — drop it (no retry)
		return fmt.Errorf("unmarshalling delivery payload: %w: %w", err, asynq.SkipRetry)
	}

	webhookID, err := uuid.Parse(p.WebhookID)
	if err != nil {
		return fmt.Errorf("parsing webhook id: %w: %w", err, asynq.SkipRetry)
	}

	log := logger.FromContext(ctx).With("webhook_id", p.WebhookID, "job_id", p.Event.JobID)

	wh, err := h.webhookRepo.GetByID(ctx, webhookID)
	if err != nil {
		if errors.Is(err, apperror.ErrWebhookNotFound) {
			// webhook was deleted between enqueue and delivery — stop retrying
			log.Warn("delivery skipped — webhook no longer exists")
			return nil
		}
		return fmt.Errorf("loading webhook: %w", err) // transient — retry
	}

	body, err := json.Marshal(p.Event)
	if err != nil {
		return fmt.Errorf("marshalling event: %w: %w", err, asynq.SkipRetry)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request: %w: %w", err, asynq.SkipRetry)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signature.Header(), signature.Sign(wh.Secret, body))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Warn("delivery failed — will retry", "url", wh.URL, "error", err.Error())
		return fmt.Errorf("delivering to %s: %w", wh.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn("delivery rejected — will retry", "url", wh.URL, "status", resp.StatusCode)
		return fmt.Errorf("webhook %s returned %d", wh.URL, resp.StatusCode)
	}

	log.Info("delivery succeeded", "url", wh.URL, "status", resp.StatusCode)
	return nil
}
