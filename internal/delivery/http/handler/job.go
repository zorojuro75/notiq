package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/contracts"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/logger"
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
	UserID      *string         `json:"user_id"      binding:"omitempty,uuid"`
	Payload     json.RawMessage `json:"payload"      binding:"required"`
	MaxRetries  int             `json:"max_retries"  binding:"gte=0,lte=100"`
	ScheduledAt *string         `json:"scheduled_at"`
}

type jobResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id,omitempty"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	RetryCount  int    `json:"retry_count"`
	MaxRetries  int    `json:"max_retries"`
	ScheduledAt string `json:"scheduled_at,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type listResponse struct {
	Jobs     []jobResponse `json:"jobs"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

func (h *JobHandler) Enqueue(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	var req enqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn("invalid enqueue request", "error", err.Error())
		response.BadRequest(c, err.Error())
		return
	}

	if !isValidJobType(req.Type) {
		log.Warn("invalid job type", "type", req.Type)
		response.BadRequest(c, "invalid job type: must be email, sms, webhook, or report")
		return
	}

	input := entity.EnqueueJobInput{
		Type:       req.Type,
		Payload:    req.Payload,
		MaxRetries: req.MaxRetries,
	}

	// optional owner — when set, terminal job events are delivered to this
	// user's registered webhooks. binding:"uuid" already validated the format.
	if req.UserID != nil && *req.UserID != "" {
		userID, err := uuid.Parse(*req.UserID)
		if err != nil {
			response.BadRequest(c, "invalid user_id")
			return
		}
		input.UserID = &userID
	}

	if key := c.GetHeader("X-Idempotency-Key"); key != "" {
		input.IdempotencyKey = &key
	}

	if req.ScheduledAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
		if err != nil {
			log.Warn("invalid scheduled_at format", "value", *req.ScheduledAt)
			response.BadRequest(c, "invalid scheduled_at: use RFC3339 format")
			return
		}
		if t.Before(time.Now().UTC()) {
			log.Warn("scheduled_at is in the past — treating as immediate")
		} else {
			input.ScheduledAt = &t
		}
	}

	out, err := h.jobUC.Enqueue(c.Request.Context(), input)
	if err != nil {
		log.Error("failed to enqueue job", "error", err.Error())
		response.InternalError(c, "failed to enqueue job")
		return
	}

	if out.Replayed {
		log.Info("idempotent replay",
			"job_id", out.Job.ID,
			"idempotency_key", input.IdempotencyKey,
		)
		c.Header("X-Idempotent-Replayed", "true")
		response.OK(c, toJobResponse(out.Job))
		return
	}

	log.Info("job enqueued",
		"job_id", out.Job.ID,
		"type", out.Job.Type,
		"scheduled_at", out.Job.ScheduledAt,
	)
	response.Created(c, toJobResponse(out.Job))
}

func (h *JobHandler) GetByID(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid job id")
		return
	}

	job, err := h.jobUC.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == apperror.ErrJobNotFound {
			response.NotFound(c, "job not found")
			return
		}
		log.Error("failed to get job", "job_id", id, "error", err.Error())
		response.InternalError(c, "failed to get job")
		return
	}

	response.OK(c, toJobResponse(job))
}

func (h *JobHandler) List(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := entity.JobFilter{}

	if s := c.Query("status"); s != "" {
		status := entity.JobStatus(s)
		filter.Status = &status
	}

	if t := c.Query("type"); t != "" {
		jobType := entity.JobType(t)
		filter.Type = &jobType
	}

	if c.Query("scheduled") == "true" {
		scheduled := true
		filter.Scheduled = &scheduled
	}

	jobs, total, err := h.jobUC.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		log.Error("failed to list jobs", "error", err.Error())
		response.InternalError(c, "failed to list jobs")
		return
	}

	result := make([]jobResponse, len(jobs))
	for i, j := range jobs {
		result[i] = toJobResponse(j)
	}

	response.OK(c, listResponse{
		Jobs:     result,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func (h *JobHandler) Cancel(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid job id")
		return
	}

	err = h.jobUC.Cancel(c.Request.Context(), id)
	if err != nil {
		switch err {
		case apperror.ErrJobNotFound:
			response.NotFound(c, "job not found")
		case apperror.ErrJobNotCancellable:
			c.JSON(http.StatusConflict, response.Response{
				Success: false,
				Error:   "job cannot be cancelled — only pending jobs can be cancelled",
			})
		default:
			log.Error("failed to cancel job", "job_id", id, "error", err.Error())
			response.InternalError(c, "failed to cancel job")
		}
		return
	}

	log.Info("job cancelled", "job_id", id)
	response.OK(c, gin.H{"message": "job cancelled"})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toJobResponse(j *entity.Job) jobResponse {
	r := jobResponse{
		ID:         j.ID.String(),
		Type:       string(j.Type),
		Status:     string(j.Status),
		RetryCount: j.RetryCount,
		MaxRetries: j.MaxRetries,
		CreatedAt:  j.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  j.UpdatedAt.Format(time.RFC3339),
	}
	if j.UserID != nil {
		r.UserID = j.UserID.String()
	}
	if j.ScheduledAt != nil {
		r.ScheduledAt = j.ScheduledAt.Format(time.RFC3339)
	}
	return r
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