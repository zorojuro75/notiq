package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zorojuro75/notiq/pkg/logger"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// use existing ID if client sent one — useful for tracing across services
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// store in context so handlers and use cases can access it
		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		// set on response so clients can reference it in bug reports
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}