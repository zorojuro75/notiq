package entity

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

type EnqueueJobInput struct {
    Type            JobType
    UserID          *uuid.UUID
    Payload         json.RawMessage
    MaxRetries      int
    IdempotencyKey  *string
    ScheduledAt     *time.Time
}

type EnqueueJobOutput struct {
    Job *Job
    Replayed bool
}

type JobFilter struct {
    Status    *JobStatus
	Type      *JobType
	Scheduled *bool
}