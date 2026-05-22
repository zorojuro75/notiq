package handlers

import (
	"context"

	"github.com/hibiken/asynq"
)

type JobHandler interface {
	Handle(ctx context.Context, task *asynq.Task) error
}