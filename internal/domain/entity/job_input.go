package entity

import (
    "encoding/json"
    "time"
)

type EnqueueJobInput struct {
    Type            JobType
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
    Status *JobStatus
    Type   *JobType
}