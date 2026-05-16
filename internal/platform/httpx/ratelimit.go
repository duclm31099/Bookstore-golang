package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitMiddleware áp dụng Fixed Window Counter giới hạn số request mỗi phút theo IP.
// Key pattern: rate:global:{ip} — khớp với quy ước SRD §15.3 (rate:{scope}:{subject}).
// Fail-Open: khi Redis lỗi, request vẫn được phép đi qua để tránh service disruption.
func RateLimitMiddleware(rdb *goredis.Client, cfg config.RateLimitConfig, log *zap.Logger) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	limit := int64(cfg.RequestPerMinute)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := "rate:global:" + ip
		ctx := c.Request.Context()

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Fail-Open: Redis lỗi không được block request hợp lệ
			log.Error("rate_limit: Redis INCR failed", zap.Error(err), zap.String("ip", ip))
			c.Next()
			return
		}

		// Chỉ set TTL khi count == 1 (request đầu tiên trong window mới).
		// Các lần INCR tiếp theo giữ nguyên TTL đã có — đây là điều kiện đúng
		// để tránh reset window liên tục khi có traffic cao.
		if count == 1 {
			rdb.Expire(ctx, key, time.Minute)
		}

		remaining := limit - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if count > limit {
			c.Header("Retry-After", "60")
			Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "too many requests, please slow down")
			return
		}

		c.Next()
	}
}
