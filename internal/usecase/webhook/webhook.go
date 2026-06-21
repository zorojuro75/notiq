package webhook

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/internal/usecase/notification"
)

type WebhookUseCase struct {
	webhookRepo repository.WebhookRepository
}

func NewWebhookUseCase(webhookRepo repository.WebhookRepository) *WebhookUseCase {
	return &WebhookUseCase{webhookRepo: webhookRepo}
}

func (uc *WebhookUseCase) Create(ctx context.Context, input entity.CreateWebhookInput) (*entity.CreateWebhookOutput, error) {
	secret, err := notification.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("generating secret: %w", err)
	}

	webhook := &entity.Webhook{
		ID:     uuid.New(),
		UserID: input.UserID,
		URL:    input.URL,
		Secret: secret,
	}

	if err := uc.webhookRepo.Create(ctx, webhook); err != nil {
		return nil, fmt.Errorf("creating webhook: %w", err)
	}

	return &entity.CreateWebhookOutput{
		Webhook: webhook,
		Secret:  secret,
	}, nil
}

func (uc *WebhookUseCase) List(ctx context.Context, userID uuid.UUID) ([]*entity.Webhook, error) {
	return uc.webhookRepo.ListByUserID(ctx, userID)
}

func (uc *WebhookUseCase) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	// The repository already returns apperror.ErrWebhookNotFound when no row
	// matches; propagate errors unchanged so real failures (e.g. DB outage)
	// aren't masked as a 404.
	return uc.webhookRepo.Delete(ctx, id, userID)
}