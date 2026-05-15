package middleware

import (
	"net/http"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports" // Import interface

	"github.com/gin-gonic/gin" // Web framework
	"go.uber.org/zap"          // Thư viện log siêu tốc
)

type StrictAuthMiddleware gin.HandlerFunc

// StrictAuthMiddleware chứa dependency cần thiết để thực hiện logic chặn request
// NewStrictAuthMiddleware khởi tạo middleware với dependency được tiêm (inject) vào
func NewStrictAuthMiddleware(blacklist ports.BlacklistPort) StrictAuthMiddleware {
	return func(c *gin.Context) { // Hàm chạy mỗi khi có request đi qua route nhạy cảm

		// Lấy jti (JWT ID) từ context. Jti này đã được AuthMiddleware cơ bản bóc tách và nhét vào trước đó
		jtiValue, exists := c.Get("jti")

		if !exists { // Nếu không tìm thấy jti (có thể do lỗi cấu hình thứ tự middleware)
			zap.L().Error("strict_auth: Missing JTI in context")                                        // Log lỗi hệ thống
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Missing JTI"}) // Chặn luôn
			return                                                                                      // Dừng xử lý
		}

		jti, ok := jtiValue.(string) // Ép kiểu biến jtiValue về dạng chuỗi (string)
		if !ok || jti == "" {        // Nếu ép kiểu thất bại hoặc jti rỗng
			zap.L().Error("strict_auth: Invalid JTI format")                                              // Log định dạng sai
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"}) // Chặn
			return
		}

		// Gọi xuống adapter (Redis) để kiểm tra xem jti này có bị cấm không
		isBlacklisted, err := blacklist.IsTokenBlacklisted(c.Request.Context(), jti)

		if err != nil { // Nếu Redis sập hoặc timeout
			zap.L().Error("strict_auth: Redis check failed", zap.Error(err), zap.String("jti", jti)) // Log lỗi nghiêm trọng
			// Quyết định an toàn: Chặn request (Fail-Closed) thay vì cho qua để bảo mật tối đa
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		if isBlacklisted { // Nếu Redis báo token này nằm trong danh sách đen
			zap.L().Warn("strict_auth: Blocked revoked token", zap.String("jti", jti)) // Cảnh báo nỗ lực xâm nhập
			// Trả mã 401 để ép client văng ra màn hình đăng nhập hoặc xin token mới
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Token revoked"})
			return
		}

		c.Next() // Mọi thứ hoàn hảo, cho phép request đi tiếp vào Handler (đặt hàng, đổi pass...)
	}
}
