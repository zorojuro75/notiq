package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
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
	job, err := h.Prepare(ctx, task)
	if err != nil {
		if err == apperror.ErrJobCancelled {
			log.Printf("job was cancelled, skipping task type: %s", task.Type())
			return nil
		}
		return fmt.Errorf("preparing job: %w", err)
	}

	var p emailPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		_ = h.FailOrDead(ctx, job)
		return fmt.Errorf("decoding email payload: %w", err)
	}

	log.Printf("[EMAIL] to=%s subject=%s body=%s", p.To, p.Subject, p.Body)

	if err := h.Complete(ctx, job.ID); err != nil {
		return fmt.Errorf("completing job: %w", err)
	}

	log.Printf("[EMAIL] job %s done", job.ID)
	return nil
}