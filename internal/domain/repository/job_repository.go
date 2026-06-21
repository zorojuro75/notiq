package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
)

type JobRepository interface {
    Create(ctx context.Context, job *entity.Job) error
    GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error)
    GetByIdempotencyKey(ctx context.Context, key string) (*entity.Job, error)
    List(ctx context.Context, filter entity.JobFilter, page, pageSize int) ([]*entity.Job, int64, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status entity.JobStatus) error
    UpdateRetryCount(ctx context.Context, id uuid.UUID, retryCount int) error
    // Delete permanently removes a job row. It is a hard delete so the
    // idempotency key is freed for a clean retry — a soft delete would leave
    // the unique index occupied.
    Delete(ctx context.Context, id uuid.UUID) error
}