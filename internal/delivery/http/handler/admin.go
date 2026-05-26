package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/usecase/admin"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/response"
)

type AdminHandler struct {
	adminUC *admin.AdminUseCase
}

func NewAdminHandler(adminUC *admin.AdminUseCase) *AdminHandler {
	return &AdminHandler{adminUC: adminUC}
}

func (h *AdminHandler) Stats(c *gin.Context) {
	stats, err := h.adminUC.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to get stats")
		return
	}
	response.OK(c, stats)
}

func (h *AdminHandler) RetryJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid job id")
		return
	}

	job, err := h.adminUC.RetryDeadJob(c.Request.Context(), id)
	if err != nil {
		switch err {
		case apperror.ErrJobNotFound:
			response.NotFound(c, "job not found")
		case apperror.ErrJobNotRetryable:
			c.JSON(http.StatusConflict, response.Response{
				Success: false,
				Error:   "only dead jobs can be manually retried",
			})
		default:
			response.InternalError(c, "failed to retry job")
		}
		return
	}

	response.OK(c, toJobResponse(job))
}