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
    Type           JobType
    Payload        json.RawMessage
    Status         JobStatus
    RetryCount     int
    MaxRetries     int
    ScheduledAt    *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}