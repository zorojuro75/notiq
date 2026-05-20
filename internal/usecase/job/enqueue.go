package job

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
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
	if input.MaxRetries == 0 {
		input.MaxRetries = 3
	}

	job := &entity.Job{
		ID:          uuid.New(),
		Type:        input.Type,
		Payload:     input.Payload,
		Status:      entity.JobStatusPending,
		RetryCount:  0,
		MaxRetries:  input.MaxRetries,
		ScheduledAt: input.ScheduledAt,
	}

	if err := uc.jobRepo.Create(ctx, job); err != nil {
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

	return &entity.EnqueueJobOutput{Job: job}, nil
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