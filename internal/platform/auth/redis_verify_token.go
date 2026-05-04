package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const emailVerifyPrefix = "identity:email_verify:"
const emailVerifyTTL = 24 * time.Hour

// RedisVerificationTokenService lưu và xác thực email verification token trong Redis.
// Token là single-use: GetDel đảm bảo không dùng lại được (chống replay attack).
type RedisVerificationTokenService struct {
	rdb *goredis.Client
}

func NewRedisVerificationTokenService(rdb *goredis.Client) *RedisVerificationTokenService {
	return &RedisVerificationTokenService{rdb: rdb}
}

// IssueEmailVerificationToken tạo random token, lưu vào Redis với TTL 24h.
func (s *RedisVerificationTokenService) IssueEmailVerificationToken(ctx context.Context, userID int64) (string, error) {
	rawToken, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("generate verify token: %w", err)
	}

	// Tối ưu Master: Ép kiểu int64 sang chuỗi base 10 siêu tốc, không cần cấp phát object hay json marshal
	userIDStr := strconv.FormatInt(userID, 10)
	key := emailVerifyPrefix + rawToken

	// Thư viện go-redis hỗ trợ truyền trực tiếp string làm value
	if err := s.rdb.Set(ctx, key, userIDStr, emailVerifyTTL).Err(); err != nil {
		return "", fmt.Errorf("store verify token: %w", err)
	}

	return rawToken, nil
}

// ParseEmailVerificationToken lấy và xóa token khỏi Redis trong một thao tác (GetDel).
func (s *RedisVerificationTokenService) ParseEmailVerificationToken(ctx context.Context, token string) (int64, error) {
	key := emailVerifyPrefix + token

	// Sử dụng Result() thay vì Bytes() để lấy thẳng ra chuỗi string
	userIDStr, err := s.rdb.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, fmt.Errorf("verification token not found or already used")
		}
		return 0, fmt.Errorf("get verify token: %w", err)
	}

	// Parse chuỗi về lại int64
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse user id from redis value: %w", err)
	}

	return userID, nil
}
