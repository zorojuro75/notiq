package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"gorm.io/gorm"
)

type Webhook struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	URL       string         `gorm:"type:varchar(500);not null"`
	Secret    string         `gorm:"type:varchar(255);not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (Webhook) TableName() string {
	return "webhooks"
}

func (m *Webhook) ToEntity() *entity.Webhook {
	return &entity.Webhook{
		ID:        m.ID,
		UserID:    m.UserID,
		URL:       m.URL,
		Secret:    m.Secret,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func FromWebhookEntity(e *entity.Webhook) *Webhook {
	return &Webhook{
		ID:        e.ID,
		UserID:    e.UserID,
		URL:       e.URL,
		Secret:    e.Secret,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}