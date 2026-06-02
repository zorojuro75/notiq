package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
)

type emailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type EmailHandler struct {
	BaseHandler
}

func NewEmailHandler(jobRepo repository.JobRepository) *EmailHandler {
	return &EmailHandler{
		BaseHandler: NewBaseHandler(jobRepo),
	}
}

func (h *EmailHandler) Handle(ctx context.Context, task *asynq.Task) error {
	job, ctx, err := h.Prepare(ctx, task)
	if err != nil {
		if err == apperror.ErrJobCancelled {
			return nil
		}
		return fmt.Errorf("preparing job: %w", err)
	}

	// inject job ID into context for all subsequent logs
	ctx = logger.WithJobID(ctx, job.ID.String())
	log := logger.FromContext(ctx)

	var p emailPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("decoding email payload: %w", err)
	}

	log.Info("sending email", "to", p.To, "subject", p.Subject)

	if err := h.Complete(ctx, job); err != nil {
		return fmt.Errorf("completing job: %w", err)
	}

	log.Info("email job done")
	return nil
}