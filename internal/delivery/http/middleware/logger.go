package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zorojuro75/notiq/pkg/logger"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// process request
		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		log := logger.FromContext(c.Request.Context())

		// choose level based on status code
		switch {
		case status >= 500:
			log.Error("request completed",
				"method", c.Request.Method,
				"path", path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"client_ip", c.ClientIP(),
			)
		case status >= 400:
			log.Warn("request completed",
				"method", c.Request.Method,
				"path", path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"client_ip", c.ClientIP(),
			)
		default:
			log.Info("request completed",
				"method", c.Request.Method,
				"path", path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
			)
		}
	}
}
