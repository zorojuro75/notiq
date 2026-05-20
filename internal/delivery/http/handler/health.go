package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func (h *HealthHandler) Check(c *gin.Context) {
	status := gin.H{
		"postgres": "ok",
		"redis":    "ok",
	}
	httpStatus := http.StatusOK

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(context.Background()) != nil {
		status["postgres"] = "unreachable"
		httpStatus = http.StatusServiceUnavailable
	}

	if err := h.redis.Ping(context.Background()).Err(); err != nil {
		status["redis"] = "unreachable"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}
