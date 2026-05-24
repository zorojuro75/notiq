package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
)

type WebhookRepository interface {
	Create(ctx context.Context, webhook *entity.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Webhook, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Webhook, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}