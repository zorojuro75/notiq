package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/internal/usecase/notification"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/safehttp"
)

type webhookPayload struct {
	URL   string          `json:"url"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type WebhookHandler struct {
	BaseHandler
	httpClient *http.Client
}

// NewWebhookHandler builds the handler for user-submitted webhook jobs. The URL
// in the payload is user-controlled, so the HTTP client is hardened against SSRF
// via safehttp. allowPrivateTargets disables that guard for local testing only.
func NewWebhookHandler(jobRepo repository.JobRepository, dispatcher *notification.Dispatcher, allowPrivateTargets bool) *WebhookHandler {
	return &WebhookHandler{
		BaseHandler: NewBaseHandler(jobRepo, dispatcher),
		httpClient:  safehttp.NewClient(10*time.Second, allowPrivateTargets),
	}
}

func (h *WebhookHandler) Handle(ctx context.Context, task *asynq.Task) error {
	job, ctx, err := h.Prepare(ctx, task)
	if err != nil {
		if err == apperror.ErrJobCancelled {
			log.Printf("job was cancelled, skipping task type: %s", task.Type())
			return nil
		}
		return fmt.Errorf("preparing job: %w", err)
	}

	var p webhookPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("decoding webhook payload: %w", err)
	}

	body, _ := json.Marshal(map[string]any{
		"event":  p.Event,
		"data":   p.Data,
		"job_id": job.ID.String(),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewBuffer(body))
	if err != nil {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("calling webhook url %s: %w", p.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}

	if err := h.Complete(ctx, job); err != nil {
		return fmt.Errorf("completing job: %w", err)
	}

	log.Printf("[WEBHOOK] job %s done — %s responded %d", job.ID, p.URL, resp.StatusCode)
	return nil
}