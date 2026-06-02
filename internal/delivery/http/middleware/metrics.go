package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zorojuro75/notiq/pkg/metrics"
)

// Metrics records Prometheus metrics for every HTTP request.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// use route pattern not actual path — prevents high cardinality
		// e.g. /api/v1/jobs/:id not /api/v1/jobs/abc-123-def-456
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		metrics.RecordHTTPRequest(
			c.Request.Method,
			path,
			status,
			duration,
		)
	}
}