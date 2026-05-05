// internal/platform/httpx/middleware/request_id.go
package httpx

import (
	util "github.com/duclm99/bookstore-backend-v2/internal/platform/idempotency"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set("request_id", requestID)
		c.Request = c.Request.WithContext(util.WithRequestIdKey(c.Request.Context(), requestID))
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}
