package http

import (
    "github.com/gin-gonic/gin"
    "github.com/zorojuro75/notiq/internal/delivery/http/handler"
)

func NewRouter(
    healthHandler *handler.HealthHandler,
    jobHandler *handler.JobHandler,
) *gin.Engine {
    r := gin.Default()

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
    }

    return r
}