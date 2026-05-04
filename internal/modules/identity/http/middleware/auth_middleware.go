package middleware

import (
	"net/http"
	"strings"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/gin-gonic/gin"
)

// ctxKey là key dùng để lưu AuthContext vào gin.Context.
// Dùng unexported type để tránh collision với các package khác.
const authContextKey = "auth_context"

// AuthContext chứa thông tin identity đã được xác thực từ access token.
// Được inject vào gin.Context bởi AuthMiddleware và đọc ra bởi GetAuthContext.
type AuthContext struct {
	UserID int64
	Email  string
	Role   string
	Type   string
}

// NewAuthMiddleware trả về Gin middleware xác thực Bearer JWT.
//
// Cơ chế:
//  1. Đọc header "Authorization: Bearer <token>"
//  2. Gọi authManager.ValidateAccessToken để verify chữ ký + expiry + type="access"
//  3. Nếu hợp lệ → lưu AuthContext vào gin.Context rồi c.Next()
//  4. Nếu không hợp lệ → 401 JSON + c.Abort() (không gọi handler tiếp theo)
func NewAuthMiddleware(authManager *auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Lấy Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_TOKEN",
					"message": "authorization header is required",
				},
			})
			return
		}

		// Chỉ chấp nhận "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN_FORMAT",
					"message": "authorization header must be 'Bearer <token>'",
				},
			})
			return
		}

		rawToken := strings.TrimSpace(parts[1])

		// Validate JWT: verify signature, expiry, type="access"
		claims, err := authManager.ValidateAccessToken(rawToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "token is invalid or expired",
				},
			})
			return
		}

		// Lưu AuthContext vào gin.Context để handler đọc ra
		c.Set(authContextKey, AuthContext{
			UserID: claims.UserID,
			Email:  claims.Email,
			Role:   claims.Role,
			Type:   claims.Type,
		})

		c.Next()
	}
}

// GetAuthContext lấy AuthContext từ gin.Context.
// Trả về (AuthContext, true) nếu middleware đã chạy và token hợp lệ.
// Trả về (AuthContext{}, false) nếu context không có — thường do route không qua middleware.
func GetAuthContext(c *gin.Context) (AuthContext, bool) {
	val, ok := c.Get(authContextKey)
	if !ok {
		return AuthContext{}, false
	}
	ac, ok := val.(AuthContext)
	return ac, ok
}
