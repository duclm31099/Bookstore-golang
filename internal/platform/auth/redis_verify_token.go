package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type verifyPayload struct {
	UserID int64 `json:"user_id"`
}

// IssueEmailVerificationToken tạo random token, lưu vào Redis với TTL 24h.
func (s *RedisVerificationTokenService) IssueEmailVerificationToken(ctx context.Context, userID int64) (string, error) {
	rawToken, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("generate verify token: %w", err)
	}

	payload, err := json.Marshal(verifyPayload{UserID: userID})
	if err != nil {
		return "", err
	}

	key := emailVerifyPrefix + rawToken
	if err := s.rdb.Set(ctx, key, payload, emailVerifyTTL).Err(); err != nil {
		return "", fmt.Errorf("store verify token: %w", err)
	}

	return rawToken, nil
}

// ParseEmailVerificationToken lấy và xóa token khỏi Redis trong một thao tác (GetDel).
func (s *RedisVerificationTokenService) ParseEmailVerificationToken(ctx context.Context, token string) (int64, error) {
	key := emailVerifyPrefix + token

	raw, err := s.rdb.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, fmt.Errorf("verification token not found or already used")
		}
		return 0, fmt.Errorf("get verify token: %w", err)
	}

	var p verifyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0, fmt.Errorf("decode verify token payload: %w", err)
	}

	return p.UserID, nil
}
