package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
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

	// inject job ID into context so all logs include it automatically
	ctx = logger.WithJobID(ctx, tp.JobID)

	log := logger.FromContext(ctx)
	log.Info("job started", "task_type", task.Type())

	job, err := b.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("fetching job: %w", err)
	}

	if job.Status == entity.JobStatusCancelled {
		log.Info("job was cancelled — skipping")
		return nil, apperror.ErrJobCancelled
	}

	if err := b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		return nil, fmt.Errorf("updating status to processing: %w", err)
	}

	log.Info("job status updated", "status", "processing")
	return job, nil
}

func (b *BaseHandler) Complete(ctx context.Context, jobID uuid.UUID) error {
	logger.FromContext(ctx).Info("job completed", "status", "done")
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusDone)
}

func (b *BaseHandler) Fail(ctx context.Context, jobID uuid.UUID, retryCount int) error {
	logger.FromContext(ctx).Warn("job failed — will retry",
		"retry_count", retryCount+1,
	)
	if err := b.jobRepo.UpdateRetryCount(ctx, jobID, retryCount+1); err != nil {
		return err
	}
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
}

func (b *BaseHandler) Dead(ctx context.Context, jobID uuid.UUID) error {
	logger.FromContext(ctx).Error("job exhausted all retries — marking dead")
	return b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusDead)
}

func (b *BaseHandler) FailOrDead(ctx context.Context, job *entity.Job) error {
	if IsLastAttempt(job) {
		return b.Dead(ctx, job.ID)
	}
	return b.Fail(ctx, job.ID, job.RetryCount)
}

func IsLastAttempt(job *entity.Job) bool {
	return job.RetryCount+1 >= job.MaxRetries
}