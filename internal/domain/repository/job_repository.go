package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
)

type JobRepository interface {
    Create(ctx context.Context, job *entity.Job) error
    GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error)
    List(ctx context.Context, filter entity.JobFilter, page, pageSize int) ([]*entity.Job, int64, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status entity.JobStatus) error
    UpdateRetryCount(ctx context.Context, id uuid.UUID, retryCount int) error
}