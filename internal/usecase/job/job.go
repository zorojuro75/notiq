package job

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
	"github.com/zorojuro75/notiq/pkg/metrics"
	"github.com/zorojuro75/notiq/pkg/queue"
)

// Enqueuer enqueues a task onto the job queue. *queue.Client satisfies it.
type Enqueuer interface {
	Enqueue(taskType string, payload any, opts queue.EnqueueOptions) error
}

// TaskCanceller removes a (scheduled) task from the queue by ID.
// *queue.Inspector satisfies it.
type TaskCanceller interface {
	DeleteTask(queueName, taskID string) error
}

type JobUseCase struct {
	jobRepo     repository.JobRepository
	queueClient Enqueuer
	inspector   TaskCanceller
}

func NewJobUseCase(
	jobRepo repository.JobRepository,
	queueClient Enqueuer,
	inspector TaskCanceller,
) *JobUseCase {
	return &JobUseCase{
		jobRepo:     jobRepo,
		queueClient: queueClient,
		inspector:   inspector,
	}
}

func (uc *JobUseCase) Enqueue(ctx context.Context, input entity.EnqueueJobInput) (*entity.EnqueueJobOutput, error) {
	if input.IdempotencyKey != nil && *input.IdempotencyKey != "" {
		existing, err := uc.jobRepo.GetByIdempotencyKey(ctx, *input.IdempotencyKey)
		if err == nil {
			return &entity.EnqueueJobOutput{
				Job:      existing,
				Replayed: true,
			}, nil
		}
		if err != apperror.ErrJobNotFound {
			return nil, fmt.Errorf("checking idempotency key: %w", err)
		}
	}

	// Defense in depth: the HTTP layer already rejects negatives, but a zero or
	// negative value reaching here would make asynq.MaxRetry(-1) and mark the
	// job dead on its first failure. Clamp to the default budget.
	if input.MaxRetries <= 0 {
		input.MaxRetries = 3
	}

	job := &entity.Job{
		ID:             uuid.New(),
		UserID:         input.UserID,
		Type:           input.Type,
		Payload:        input.Payload,
		Status:         entity.JobStatusPending,
		RetryCount:     0,
		MaxRetries:     input.MaxRetries,
		IdempotencyKey: input.IdempotencyKey,
		ScheduledAt:    input.ScheduledAt,
	}

	if err := uc.jobRepo.Create(ctx, job); err != nil {
		if err == apperror.ErrDuplicateIdempotencyKey && input.IdempotencyKey != nil {
			existing, fetchErr := uc.jobRepo.GetByIdempotencyKey(ctx, *input.IdempotencyKey)
			if fetchErr == nil {
				return &entity.EnqueueJobOutput{
					Job:      existing,
					Replayed: true,
				}, nil
			}
		}
		return nil, fmt.Errorf("saving job: %w", err)
	}

	err := uc.queueClient.Enqueue(queue.TaskTypeForJob(job.Type), map[string]any{
		"job_id":  job.ID.String(),
		"payload": job.Payload,
	}, queue.EnqueueOptions{
		MaxRetry:    job.MaxRetries,
		ScheduledAt: job.ScheduledAt,
		TaskID:      job.ID.String(),
	})
	if err != nil {
		// The job row exists but never reached the queue, so it would sit
		// "pending" forever and — worse — its idempotency key would make a
		// retry replay a job that was never processed. Compensate by removing
		// the row so the caller can retry cleanly.
		if delErr := uc.jobRepo.Delete(ctx, job.ID); delErr != nil {
			logger.FromContext(ctx).Error("failed to roll back orphaned job after enqueue error",
				"job_id", job.ID.String(), "error", delErr.Error())
		}
		return nil, fmt.Errorf("pushing to queue: %w", err)
	}

	metrics.RecordJobEnqueued(string(job.Type))
	return &entity.EnqueueJobOutput{Job: job, Replayed: false}, nil
}

func (uc *JobUseCase) GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error) {
	return uc.jobRepo.GetByID(ctx, id)
}

func (uc *JobUseCase) List(ctx context.Context, filter entity.JobFilter, page, pageSize int) ([]*entity.Job, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return uc.jobRepo.List(ctx, filter, page, pageSize)
}

func (uc *JobUseCase) Cancel(ctx context.Context, id uuid.UUID) error {
	job, err := uc.jobRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if job.Status != entity.JobStatusPending {
		return apperror.ErrJobNotCancellable
	}

	if err := uc.jobRepo.UpdateStatus(ctx, id, entity.JobStatusCancelled); err != nil {
		return err
	}

	if job.ScheduledAt != nil && job.ScheduledAt.After(time.Now().UTC()) {
		if err := uc.inspector.DeleteTask(queue.DefaultQueue, id.String()); err != nil {
			logger.FromContext(ctx).Warn("failed to remove cancelled task from Redis",
				"job_id", id.String(), "error", err.Error())
		}
	}

	return nil
}
