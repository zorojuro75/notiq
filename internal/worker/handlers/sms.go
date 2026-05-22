// internal/worker/handlers/sms.go
package handlers

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
)

type SMSHandler struct {
	BaseHandler
}

func NewSMSHandler(jobRepo repository.JobRepository) *SMSHandler {
	return &SMSHandler{BaseHandler: NewBaseHandler(jobRepo)}
}

func (h *SMSHandler) Handle(ctx context.Context, task *asynq.Task) error {
	job, err := h.Prepare(ctx, task)
	if err != nil {
		if err == apperror.ErrJobCancelled {
			return nil
		}
		return err
	}
	log.Printf("[SMS] job %s — payload: %s", job.ID, job.Payload)
	return h.Complete(ctx, job.ID)
}