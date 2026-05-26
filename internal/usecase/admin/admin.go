package admin

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/queue"
)

// QueueStats holds the combined Redis + Postgres operational snapshot.
type QueueStats struct {
	Pending   int `json:"pending"`
	Active    int `json:"active"`
	Retry     int `json:"retry"`
	Dead      int `json:"dead"`
	Scheduled int `json:"scheduled"`
	Completed int `json:"completed"`
}

type StatsOutput struct {
	Queue        QueueStats `json:"queue"`
	DeadJobsInDB int64      `json:"dead_jobs_in_db"`
}

type AdminUseCase struct {
	jobRepo     repository.JobRepository
	queueClient *queue.Client
	inspector   *queue.Inspector
}

func NewAdminUseCase(
	jobRepo repository.JobRepository,
	queueClient *queue.Client,
	inspector *queue.Inspector,
) *AdminUseCase {
	return &AdminUseCase{
		jobRepo:     jobRepo,
		queueClient: queueClient,
		inspector:   inspector,
	}
}

func (uc *AdminUseCase) GetStats(ctx context.Context) (*StatsOutput, error) {
	// get queue info from asynq Inspector
	info, err := uc.inspector.GetQueueInfo("default")
	if err != nil {
		log.Printf("[ADMIN] failed to get queue info: %v", err)
		// don't fail entirely — return what we can from Postgres
		info = &asynq.QueueInfo{}
	}

	// get dead job count from Postgres
	deadStatus := entity.JobStatusDead
	_, deadCount, err := uc.jobRepo.List(ctx, entity.JobFilter{
		Status: &deadStatus,
	}, 1, 1)
	if err != nil {
		return nil, fmt.Errorf("counting dead jobs: %w", err)
	}

	return &StatsOutput{
		Queue: QueueStats{
			Pending:   info.Pending,
			Active:    info.Active,
			Retry:     info.Retry,
			Dead:      info.Failed,
			Scheduled: info.Scheduled,
			Completed: info.Completed,
		},
		DeadJobsInDB: deadCount,
	}, nil
}

func (uc *AdminUseCase) RetryDeadJob(ctx context.Context, id uuid.UUID) (*entity.Job, error) {
	job, err := uc.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// only dead jobs can be manually retried
	if job.Status != entity.JobStatusDead {
		return nil, apperror.ErrJobNotRetryable
	}

	// reset retry count — job deserves its full budget again
	if err := uc.jobRepo.UpdateRetryCount(ctx, id, 0); err != nil {
		return nil, fmt.Errorf("resetting retry count: %w", err)
	}

	// update status to pending
	if err := uc.jobRepo.UpdateStatus(ctx, id, entity.JobStatusPending); err != nil {
		return nil, fmt.Errorf("updating status: %w", err)
	}

	// push back into Redis queue
	err = uc.queueClient.Enqueue(
		jobTypeToTaskType(job.Type),
		map[string]any{
			"job_id":  job.ID.String(),
			"payload": job.Payload,
		},
		queue.EnqueueOptions{
			MaxRetry: job.MaxRetries,
			TaskID:   job.ID.String() + "-retry",
			// note: append "-retry" to avoid asynq dedup
			// original task ID may still exist in dead set
		},
	)
	if err != nil {
		// rollback status — re-enqueue failed
		_ = uc.jobRepo.UpdateStatus(ctx, id, entity.JobStatusDead)
		_ = uc.jobRepo.UpdateRetryCount(ctx, id, job.RetryCount)
		return nil, fmt.Errorf("re-enqueueing job: %w", err)
	}

	// fetch updated job to return
	updated, err := uc.jobRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// jobTypeToTaskType mirrors the mapping in usecase/job — kept here to
// avoid importing the job use case package (would create circular deps)
func jobTypeToTaskType(t entity.JobType) string {
	switch t {
	case entity.JobTypeEmail:
		return "job:email"
	case entity.JobTypeSMS:
		return "job:sms"
	case entity.JobTypeWebhook:
		return "job:webhook"
	case entity.JobTypeReport:
		return "job:report"
	default:
		return string(t)
	}
}