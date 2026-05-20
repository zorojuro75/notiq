package contracts

import (
    "context"
    "github.com/zorojuro75/notiq/internal/domain/entity"
)

type JobUseCase interface {
    Enqueue(ctx context.Context, input entity.EnqueueJobInput) (*entity.EnqueueJobOutput, error)
    // GetByID(ctx context.Context, id uuid.UUID) (*entity.Job, error)
    // List(ctx context.Context, filter entity.JobFilter, page, pageSize int) ([]*entity.Job, int64, error)
    // Cancel(ctx context.Context, id uuid.UUID) error
}