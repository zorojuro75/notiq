package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/metrics"
)

type contextKey string

const startTimeKey contextKey = "job_start_time"

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

func (b *BaseHandler) Prepare(ctx context.Context, task *asynq.Task) (*entity.Job, context.Context, error) {
	var tp TaskPayload
	if err := json.Unmarshal(task.Payload(), &tp); err != nil {
		return nil, ctx, fmt.Errorf("unmarshalling task payload: %w", err)
	}

	jobID, err := uuid.Parse(tp.JobID)
	if err != nil {
		return nil, ctx, fmt.Errorf("parsing job id: %w", err)
	}

	ctx = logger.WithJobID(ctx, tp.JobID)
	ctx = context.WithValue(ctx, startTimeKey, time.Now())

	log := logger.FromContext(ctx)
	log.Info("job started", "task_type", task.Type())

	job, err := b.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, ctx, fmt.Errorf("fetching job: %w", err)
	}

	if job.Status == entity.JobStatusCancelled {
		log.Info("job was cancelled — skipping")
		return nil, ctx, apperror.ErrJobCancelled
	}

	if err := b.jobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		return nil, ctx, fmt.Errorf("updating status to processing: %w", err)
	}

	log.Info("job status updated", "status", "processing")
	return job, ctx, nil
}

func (b *BaseHandler) Complete(ctx context.Context, job *entity.Job) error {
	logger.FromContext(ctx).Info("job completed", "status", "done")

	if err := b.jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusDone); err != nil {
		return err
	}

	metrics.RecordJobProcessed(string(job.Type), "done", elapsedSeconds(ctx))
	return nil
}

func (b *BaseHandler) Fail(ctx context.Context, job *entity.Job) error {
	logger.FromContext(ctx).Warn("job failed — will retry",
		"retry_count", job.RetryCount+1,
	)

	if err := b.jobRepo.UpdateRetryCount(ctx, job.ID, job.RetryCount+1); err != nil {
		return err
	}
	if err := b.jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusFailed); err != nil {
		return err
	}

	metrics.RecordJobProcessed(string(job.Type), "failed", elapsedSeconds(ctx))
	return nil
}

func (b *BaseHandler) Dead(ctx context.Context, job *entity.Job) error {
	logger.FromContext(ctx).Error("job exhausted all retries — marking dead")

	if err := b.jobRepo.UpdateStatus(ctx, job.ID, entity.JobStatusDead); err != nil {
		return err
	}

	metrics.RecordJobProcessed(string(job.Type), "dead", elapsedSeconds(ctx))
	return nil
}

func (b *BaseHandler) FailOrDead(ctx context.Context, job *entity.Job) error {
	if IsLastAttempt(job) {
		return b.Dead(ctx, job)
	}
	return b.Fail(ctx, job)
}

func IsLastAttempt(job *entity.Job) bool {
	return job.RetryCount+1 >= job.MaxRetries
}

func elapsedSeconds(ctx context.Context) float64 {
	if start, ok := ctx.Value(startTimeKey).(time.Time); ok {
		return time.Since(start).Seconds()
	}
	return 0
}