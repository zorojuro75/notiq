package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/internal/domain/entity"
	"github.com/zorojuro75/notiq/internal/usecase/webhook"
	"github.com/zorojuro75/notiq/pkg/apperror"
	"github.com/zorojuro75/notiq/pkg/response"
)

type WebhookHandler struct {
	webhookUC *webhook.WebhookUseCase
}

func NewWebhookHandler(webhookUC *webhook.WebhookUseCase) *WebhookHandler {
	return &WebhookHandler{webhookUC: webhookUC}
}

type createWebhookRequest struct {
	URL    string `json:"url"     binding:"required,url"`
	UserID string `json:"user_id" binding:"required"`
}

type webhookResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type createWebhookResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Secret    string `json:"secret"`              // only in create response
	Warning   string `json:"warning"`
	CreatedAt string `json:"created_at"`
}

func (h *WebhookHandler) Create(c *gin.Context) {
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}

	out, err := h.webhookUC.Create(c.Request.Context(), entity.CreateWebhookInput{
		UserID: userID,
		URL:    req.URL,
	})
	if err != nil {
		response.InternalError(c, "failed to create webhook")
		return
	}

	c.JSON(http.StatusCreated, response.Response{
		Success: true,
		Data: createWebhookResponse{
			ID:        out.Webhook.ID.String(),
			URL:       out.Webhook.URL,
			Secret:    out.Secret,
			Warning:   "store this secret safely — it will never be shown again",
			CreatedAt: out.Webhook.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}

func (h *WebhookHandler) List(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		response.BadRequest(c, "user_id query param required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}

	webhooks, err := h.webhookUC.List(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list webhooks")
		return
	}

	result := make([]webhookResponse, len(webhooks))
	for i, wh := range webhooks {
		result[i] = webhookResponse{
			ID:        wh.ID.String(),
			URL:       wh.URL,
			CreatedAt: wh.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	response.OK(c, result)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid webhook id")
		return
	}

	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		response.BadRequest(c, "user_id query param required")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.BadRequest(c, "invalid user_id")
		return
	}

	if err := h.webhookUC.Delete(c.Request.Context(), id, userID); err != nil {
		if err == apperror.ErrWebhookNotFound {
			response.NotFound(c, "webhook not found")
			return
		}
		response.InternalError(c, "failed to delete webhook")
		return
	}

	response.OK(c, gin.H{"message": "webhook deleted"})
}