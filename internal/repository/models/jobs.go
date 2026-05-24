package models

import (
    "encoding/json"
    "time"

	"github.com/zorojuro75/notiq/internal/domain/entity"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

type Job struct {
    ID          uuid.UUID       `gorm:"type:uuid;primaryKey"`
    Type        string          `gorm:"type:varchar(50);not null;index"`
    Payload     json.RawMessage `gorm:"type:jsonb;not null"`
    Status      string          `gorm:"type:varchar(20);not null;index"`
    RetryCount  int             `gorm:"not null;default:0"`
    MaxRetries  int             `gorm:"not null;default:3"`
    IdempotencyKey *string      `gorm:"type:varchar(255);uniqueIndex"`
    ScheduledAt *time.Time      `gorm:"index"`
    CreatedAt   time.Time       `gorm:"index"`
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt  `gorm:"index"`
}

func (Job) TableName() string {
    return "jobs"
}

// ToEntity converts a DB model to a domain entity
func (m *Job) ToEntity() *entity.Job {
	return &entity.Job{
		ID:             m.ID,
		Type:           entity.JobType(m.Type),
		Payload:        m.Payload,
		Status:         entity.JobStatus(m.Status),
		RetryCount:     m.RetryCount,
		MaxRetries:     m.MaxRetries,
		IdempotencyKey: m.IdempotencyKey,
		ScheduledAt:    m.ScheduledAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

// FromEntity converts a domain entity to a DB model
func FromJobEntity(e *entity.Job) *Job {
	return &Job{
		ID:             e.ID,
		Type:           string(e.Type),
		Payload:        e.Payload,
		Status:         string(e.Status),
		RetryCount:     e.RetryCount,
		MaxRetries:     e.MaxRetries,
		IdempotencyKey: e.IdempotencyKey,
		ScheduledAt:    e.ScheduledAt,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}