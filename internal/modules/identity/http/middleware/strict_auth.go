package middleware

import (
	"net/http"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type StrictAuthMiddleware gin.HandlerFunc

// NewStrictAuthMiddleware kiểm tra JTI của access token có trong blacklist Redis không.
// Phải chạy SAU AuthMiddleware (vì cần AuthContext đã được set sẵn trong context).
func NewStrictAuthMiddleware(blacklist ports.BlacklistPort) StrictAuthMiddleware {
	return func(c *gin.Context) {
		// Đọc JTI từ AuthContext — đã được AuthMiddleware bóc tách và lưu vào context
		ac, ok := GetAuthContext(c)
		if !ok || ac.JTI == "" {
			zap.L().Error("strict_auth: Missing auth context or JTI")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Missing JTI"})
			return
		}

		isBlacklisted, err := blacklist.IsTokenBlacklisted(c.Request.Context(), ac.JTI)
		if err != nil {
			// Fail-Closed: Redis lỗi → chặn request, không cho đi qua
			zap.L().Error("strict_auth: Redis check failed", zap.Error(err), zap.String("jti", ac.JTI))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		if isBlacklisted {
			zap.L().Warn("strict_auth: Blocked revoked token", zap.String("jti", ac.JTI))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Token revoked"})
			return
		}

		c.Next()
	}
}
