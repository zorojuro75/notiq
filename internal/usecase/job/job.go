package job

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/zorojuro75/notiq/internal/domain/entity"
    "github.com/zorojuro75/notiq/internal/domain/repository"
    "github.com/zorojuro75/notiq/pkg/apperror"
    "github.com/zorojuro75/notiq/pkg/queue"
)

type JobUseCase struct {
    jobRepo     repository.JobRepository
    queueClient *queue.Client
}

func NewJobUseCase(
    jobRepo repository.JobRepository,
    queueClient *queue.Client,
) *JobUseCase {
    return &JobUseCase{
        jobRepo:     jobRepo,
        queueClient: queueClient,
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

	if input.MaxRetries == 0 {
		input.MaxRetries = 3
	}

	job := &entity.Job{
		ID:             uuid.New(),
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

	err := uc.queueClient.Enqueue(jobTypeToTaskType(job.Type), map[string]any{
		"job_id":  job.ID.String(),
		"payload": job.Payload,
	}, queue.EnqueueOptions{
		MaxRetry:    job.MaxRetries,
		ScheduledAt: job.ScheduledAt,
		TaskID:      job.ID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("pushing to queue: %w", err)
	}

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
    return uc.jobRepo.UpdateStatus(ctx, id, entity.JobStatusCancelled)
}

func jobTypeToTaskType(t entity.JobType) string {
    switch t {
    case entity.JobTypeEmail:
        return queue.TypeEmail
    case entity.JobTypeSMS:
        return queue.TypeSMS
    case entity.JobTypeWebhook:
        return queue.TypeWebhook
    case entity.JobTypeReport:
        return queue.TypeReport
    default:
        return string(t)
    }
}