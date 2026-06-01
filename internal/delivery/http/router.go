package http

import (
	"github.com/gin-gonic/gin"
	"github.com/zorojuro75/notiq/internal/delivery/http/handler"
	"github.com/zorojuro75/notiq/internal/delivery/http/middleware"
)

func NewRouter(
    healthHandler   *handler.HealthHandler,
    jobHandler      *handler.JobHandler,
    webhookHandler  *handler.WebhookHandler,
    adminHandler    *handler.AdminHandler,
	adminUsername   string,
	adminPassword   string,
) *gin.Engine {
    r := gin.Default()

    r.Use(middleware.RequestID())

    r.Use(middleware.Logger())

    r.Use(gin.Recovery())

    r.GET("/healthz", healthHandler.Check)

    v1 := r.Group("/api/v1")
    {
        jobs := v1.Group("/jobs")
        {
            jobs.POST("", jobHandler.Enqueue)
            jobs.GET("", jobHandler.List)
            jobs.GET("/:id", jobHandler.GetByID)
            jobs.DELETE("/:id", jobHandler.Cancel)
        }

        webhooks := v1.Group("/webhooks")
        {
            webhooks.POST("", webhookHandler.Create)
            webhooks.GET("", webhookHandler.List)
            webhooks.DELETE("/:id", webhookHandler.Delete)
        }

        // admin routes — protected by basic auth
		adminGroup := v1.Group("/admin")
		adminGroup.Use(middleware.BasicAuth(adminUsername, adminPassword))
		{
			adminGroup.GET("/stats", adminHandler.Stats)
			adminGroup.POST("/jobs/:id/retry", adminHandler.RetryJob)
		}
    }

    return r
}