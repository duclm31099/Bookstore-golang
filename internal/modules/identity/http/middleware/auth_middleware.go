package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ctxKey là key dùng để lưu AuthContext vào gin.Context.
// Dùng unexported type để tránh collision với các package khác.
const authContextKey = "auth_context"
const AuthorizationHeader = "Authorization"

const (
	ERROR_MISSING_TOKEN        = "MISSING_TOKEN"
	ERROR_INVALID_TOKEN_FORMAT = "INVALID_TOKEN_FORMAT"
	ERROR_INVALID_TOKEN        = "INVALID_TOKEN"

	MESSAGE_MISSING_TOKEN        = "authorization header is required"
	MESSAGE_INVALID_TOKEN_FORMAT = "authorization header must be 'Bearer <token>'"
	MESSAGE_INVALID_TOKEN        = "token is invalid or expired"
)

// AuthContext chứa thông tin identity đã được xác thực từ access token.
// Được inject vào gin.Context bởi AuthMiddleware và đọc ra bởi GetAuthContext.
type AuthContext struct {
	UserID    int64
	Email     string
	Role      string
	DeviceID  int64
	SessionID int64
	JTI       string
}
type AuthMiddleware gin.HandlerFunc

// NewAuthMiddleware trả về Gin middleware xác thực Bearer JWT.
//
// Cơ chế:
//  1. Đọc header "Authorization: Bearer <token>"
//  2. Gọi authManager.ValidateAccessToken để verify chữ ký + expiry + type="access"
//  3. Nếu hợp lệ → lưu AuthContext vào gin.Context rồi c.Next()
//  4. Nếu không hợp lệ → 401 JSON + c.Abort() (không gọi handler tiếp theo)
func NewAuthMiddleware(authManager *auth.Auth) AuthMiddleware {
	return func(c *gin.Context) {
		// Lấy Authorization header
		authHeader := c.GetHeader(AuthorizationHeader)
		zap.L().Info("auth header", zap.String("header", authHeader))
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    ERROR_MISSING_TOKEN,
					"message": MESSAGE_MISSING_TOKEN,
				},
			})

			return
		}

		// Chỉ chấp nhận "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    ERROR_INVALID_TOKEN_FORMAT,
					"message": MESSAGE_INVALID_TOKEN_FORMAT,
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
					"code":    ERROR_INVALID_TOKEN,
					"message": MESSAGE_INVALID_TOKEN,
				},
			})
			return
		}
		zap.L().Info("token is valid",
			zap.Int64("user_id", claims.UserID),
			zap.String("email", claims.Email),
			zap.String("role", claims.Role),
			zap.String("session_id", claims.SessionID),
			zap.String("device_id", claims.DeviceID),
		)

		// Lưu AuthContext vào gin.Context để handler đọc ra
		sid, _ := strconv.ParseInt(claims.SessionID, 10, 64)
		did, _ := strconv.ParseInt(claims.DeviceID, 10, 64)
		c.Set(authContextKey, AuthContext{
			UserID:    claims.UserID,
			Email:     claims.Email,
			Role:      claims.Role,
			SessionID: sid,
			DeviceID:  did,
			JTI:       claims.RegisteredClaims.ID,
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
