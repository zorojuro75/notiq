package entity

import (
	"github.com/google/uuid"
)

type CreateWebhookInput struct {
	UserID uuid.UUID
	URL    string
}

type CreateWebhookOutput struct {
	Webhook *Webhook
	Secret  string
}