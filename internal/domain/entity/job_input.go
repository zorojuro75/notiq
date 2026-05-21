package entity

import (
    "encoding/json"
    "time"
)

type EnqueueJobInput struct {
    Type        JobType
    Payload     json.RawMessage
    MaxRetries  int
    ScheduledAt *time.Time
}

type EnqueueJobOutput struct {
    Job *Job
}

type JobFilter struct {
    Status *JobStatus
    Type   *JobType
}