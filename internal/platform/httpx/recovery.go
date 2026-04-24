// internal/platform/httpx/middleware/recovery.go
package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RecoveryMiddleware(log *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error("panic recovered", zap.Any("panic", recovered))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "internal server error",
			},
		})
	})
}
