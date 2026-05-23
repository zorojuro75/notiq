package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
)

type TaskPayload struct {
	JobID   string          `json:"job_id"`
	Payload json.RawMessage `json:"payload"`
}

type BaseHandler struct {
	jobRepo repository.JobRepository
}

func NewBaseHandler(jobRepo repository.JobRepository) BaseHandler {
	return BaseHandler{jobRepo: jobRepo}
}

func (b *BaseHandler) Prepare(ctx context.Context, task *asynq.Task) (*entity.Job, error) {
	var tp TaskPayload
	if err := json.Unmarshal(task.Payload(), &tp); err != nil {
		return nil, fmt.Errorf("unmarshalling task payload: %w", err)
	}

	jobID, err := uuid.Parse(tp.JobID)
	if err != nil {
		return nil, fmt.Errorf("parsing job id: %w", err)
	}

	job, err := b.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("fetching job: %w", err)
	}

	if job.Status == entity.JobStatusCancelled {
		return nil, apperror.ErrJobCancelled
	}

	if err := b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		return nil, fmt.Errorf("updating status to processing: %w", err)
	}

	return job, nil
}

func (b *BaseHandler) Complete(ctx context.Context, jobID uuid.UUID) error {
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusDone)
}

func (b *BaseHandler) Fail(ctx context.Context, jobID uuid.UUID, retryCount int) error {
	if err := b.jobRepo.UpdateRetryCount(ctx, jobID, retryCount+1); err != nil {
		return err
	}
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
}

func (b *BaseHandler) Dead(ctx context.Context, jobID uuid.UUID) error {
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusDead)
}
func (b *BaseHandler) FailOrDead(ctx context.Context, job *entity.Job) error {
	if IsLastAttempt(job) {
		log.Printf("[DEAD] job %s exhausted all %d retries", job.ID, job.MaxRetries)
		return b.Dead(ctx, job.ID)
	}
	log.Printf("[FAILED] job %s attempt %d of %d — will retry",
		job.ID, job.RetryCount+1, job.MaxRetries)
	return b.Fail(ctx, job.ID, job.RetryCount)
}

func IsLastAttempt(job *entity.Job) bool {
	return job.RetryCount+1 >= job.MaxRetries
}