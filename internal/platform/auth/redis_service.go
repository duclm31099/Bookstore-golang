package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	go_redis "github.com/redis/go-redis/v9"
)

const emailVerifyPrefix = "identity:email_verify:"
const emailVerifyTTL = 24 * time.Hour

// IssueVerifyToken tạo random token, lưu vào Redis với TTL 24h.
func (s *RedisSessionService) IssueVerifyToken(ctx context.Context, userID int64) (string, error) {
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

// ParseVerifyToken lấy và xóa token khỏi Redis trong một thao tác (GetDel).
func (s *RedisSessionService) ParseVerifyToken(ctx context.Context, token string) (int64, error) {
	key := emailVerifyPrefix + token

	// Sử dụng Result() thay vì Bytes() để lấy thẳng ra chuỗi string
	// GetDel: Get value of key and delete the key (atomic)
	userIDStr, err := s.rdb.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, go_redis.Nil) {
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

// SESSION REDIS SERVICE

type RedisSessionService struct {
	rdb *go_redis.Client
}

func NewRedisSessionService(rdb *go_redis.Client) *RedisSessionService {
	return &RedisSessionService{rdb: rdb}
}

func (s *RedisSessionService) DeleteSession(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, key).Err()
}

func (s *RedisSessionService) SetUserSession(ctx context.Context, key string, value any, TTL int64) error {
	return s.rdb.Set(ctx, key, value, time.Duration(TTL)*time.Hour).Err()
}

func (s *RedisSessionService) GetUserSession(ctx context.Context, key string) (any, error) {
	return s.rdb.Get(ctx, key).Result()
}

func (r *RedisSessionService) DeleteMultipleSessions(ctx context.Context, keys []string) error {
	// Rất quan trọng: Check slice rỗng để tránh gọi Redis vô ích
	if len(keys) == 0 {
		return nil
	}

	// redisClient.Del nhận vào variadic arguments (...string)
	// Lệnh này trả về số lượng key thực sự bị xóa và error
	_, err := r.rdb.Del(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("redis delete multiple sessions failed: %w", err)
	}

	// (Tuỳ chọn) Bạn có thể log số lượng key đã xóa để debug
	// zap.L().Debug("Deleted multiple sessions", zap.Int64("count", deletedCount))

	return nil
}
