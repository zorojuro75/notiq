// internal/worker/handlers/report.go
package handlers

import (
	"context"
	"log"

	"github.com/hibiken/asynq"
	"github.com/zorojuro75/notiq/internal/domain/repository"
	"github.com/zorojuro75/notiq/pkg/apperror"
)

type ReportHandler struct {
	BaseHandler
}

func NewReportHandler(jobRepo repository.JobRepository) *ReportHandler {
	return &ReportHandler{BaseHandler: NewBaseHandler(jobRepo)}
}

func (h *ReportHandler) Handle(ctx context.Context, task *asynq.Task) error {
	job, err := h.Prepare(ctx, task)
	if err != nil {
		if err == apperror.ErrJobCancelled {
			return nil
		}
		return err
	}
	log.Printf("[REPORT] job %s — payload: %s", job.ID, job.Payload)
	return h.Complete(ctx, job.ID)
}