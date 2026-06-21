package entity

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

type JobStatus string

const (
    JobStatusPending    JobStatus = "pending"
    JobStatusProcessing JobStatus = "processing"
    JobStatusDone       JobStatus = "done"
    JobStatusFailed     JobStatus = "failed"
    JobStatusDead       JobStatus = "dead"
    JobStatusCancelled  JobStatus = "cancelled"
)

type JobType string

const (
    JobTypeEmail   JobType = "email"
    JobTypeSMS     JobType = "sms"
    JobTypeWebhook JobType = "webhook"
    JobTypeReport  JobType = "report"
)

type Job struct {
    ID             uuid.UUID
    UserID         *uuid.UUID // owner — when set, terminal events are delivered to this user's webhooks
    Type           JobType
    Payload        json.RawMessage
    Status         JobStatus
    RetryCount     int
    MaxRetries     int
    IdempotencyKey *string
    ScheduledAt    *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}