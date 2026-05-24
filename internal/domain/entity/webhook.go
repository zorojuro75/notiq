package entity

import (
	"time"

	"github.com/google/uuid"
)

type Webhook struct {
	ID        uuid.UUID
	UserID    uuid.UUID  // which client owns this webhook
	URL       string
	Secret    string     // used to sign deliveries — returned once at creation
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WebhookDeliveryEvent struct {
	JobID     string `json:"job_id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}