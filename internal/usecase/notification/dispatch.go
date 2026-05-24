package notification

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/signature"
)

type DispatchUseCase struct {
	webhookRepo repository.WebhookRepository
	httpClient  *http.Client
}

func NewDispatchUseCase(webhookRepo repository.WebhookRepository) *DispatchUseCase {
	return &DispatchUseCase{
		webhookRepo: webhookRepo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Dispatch sends a webhook notification to all registered URLs for a user.
// Called after a job reaches a terminal state (done, failed, dead).
func (uc *DispatchUseCase) Dispatch(ctx context.Context, userID uuid.UUID, job *entity.Job) {
	webhooks, err := uc.webhookRepo.ListByUserID(ctx, userID)
	if err != nil {
		log.Printf("[DISPATCH] failed to list webhooks for user %s: %v", userID, err)
		return
	}

	if len(webhooks) == 0 {
		return
	}

	event := entity.WebhookDeliveryEvent{
		JobID:     job.ID.String(),
		Type:      string(job.Type),
		Status:    string(job.Status),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("[DISPATCH] failed to marshal event: %v", err)
		return
	}

	// deliver to every registered webhook URL
	for _, wh := range webhooks {
		go uc.deliver(ctx, wh, payload)
	}
}

func (uc *DispatchUseCase) deliver(ctx context.Context, wh *entity.Webhook, payload []byte) {
	sig := signature.Sign(wh.Secret, payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("[DISPATCH] failed to build request for webhook %s: %v", wh.ID, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signature.Header(), sig)

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		log.Printf("[DISPATCH] delivery failed for webhook %s url=%s: %v", wh.ID, wh.URL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[DISPATCH] webhook %s url=%s returned %d", wh.ID, wh.URL, resp.StatusCode)
		return
	}

	log.Printf("[DISPATCH] webhook %s delivered successfully — status %d", wh.ID, resp.StatusCode)
}

// GenerateSecret creates a cryptographically random webhook secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("generating secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}