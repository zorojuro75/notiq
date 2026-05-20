package http

import (
	"github.com/gin-gonic/gin"
	"github.com/zorojuro75/notiq/internal/delivery/http/handler"
)

func NewRouter(healthHandler *handler.HealthHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/healthz", healthHandler.Check)

	v1 := r.Group("/api/v1")
	_ = v1

	return r
}
