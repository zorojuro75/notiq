package handler

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zorojuro75/notiq/internal/domain/contracts"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/pkg/response"
)

type JobHandler struct {
	jobUC contracts.JobUseCase
}

func NewJobHandler(jobUC contracts.JobUseCase) *JobHandler {
	return &JobHandler{jobUC: jobUC}
}

type enqueueRequest struct {
	Type        entity.JobType  `json:"type"         binding:"required"`
	Payload     json.RawMessage `json:"payload"      binding:"required"`
	MaxRetries  int             `json:"max_retries"`
	ScheduledAt *string         `json:"scheduled_at"`
}

type jobResponse struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	MaxRetries  int     `json:"max_retries"`
	CreatedAt   string  `json:"created_at"`
}

func (h *JobHandler) Enqueue(c *gin.Context) {
	var req enqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if !isValidJobType(req.Type) {
		response.BadRequest(c, "invalid job type: must be email, sms, webhook, or report")
		return
	}

	input := entity.EnqueueJobInput{
		Type:       req.Type,
		Payload:    req.Payload,
		MaxRetries: req.MaxRetries,
	}

	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err != nil {
			response.BadRequest(c, "invalid scheduled_at format, use RFC3339: 2026-05-20T15:00:00Z")
			return
		}
		input.ScheduledAt = &t
	}

	out, err := h.jobUC.Enqueue(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "failed to enqueue job")
		return
	}

	response.Created(c, jobResponse{
		ID:         out.Job.ID.String(),
		Type:       string(out.Job.Type),
		Status:     string(out.Job.Status),
		MaxRetries: out.Job.MaxRetries,
		CreatedAt:  out.Job.CreatedAt.Format(time.RFC3339),
	})
}

func isValidJobType(t entity.JobType) bool {
	switch t {
	case entity.JobTypeEmail,
		entity.JobTypeSMS,
		entity.JobTypeWebhook,
		entity.JobTypeReport:
		return true
	}
	return false
}