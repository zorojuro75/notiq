package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/zorojuro75/notiq/internal/domain/entity"
	domainrepo "github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/internal/repository/models"
	"github.com/zorojuro75/notiq/pkg/apperror"
)

type webhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) domainrepo.WebhookRepository {
	return &webhookRepository{db: db}
}

func (r *webhookRepository) Create(ctx context.Context, webhook *entity.Webhook) error {
	if webhook.ID == uuid.Nil {
		webhook.ID = uuid.New()
	}
	m := models.FromWebhookEntity(webhook)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	webhook.CreatedAt = m.CreatedAt
	webhook.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *webhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Webhook, error) {
	var m models.Webhook
	err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrWebhookNotFound
		}
		return nil, err
	}
	return m.ToEntity(), nil
}

func (r *webhookRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Webhook, error) {
	var ms []models.Webhook
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&ms).Error; err != nil {
		return nil, err
	}

	webhooks := make([]*entity.Webhook, len(ms))
	for i, m := range ms {
		m := m
		webhooks[i] = m.ToEntity()
	}
	return webhooks, nil
}

func (r *webhookRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Webhook{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperror.ErrWebhookNotFound
	}
	return nil
}