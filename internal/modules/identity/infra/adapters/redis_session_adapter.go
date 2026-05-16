package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/cryptoutil"
	go_redis "github.com/redis/go-redis/v9"
)

const sessionPrefix = "identity:refresh_token:"
const resetPasswordPrefix = "identity:reset_password:"
const emailVerifyPrefix = "identity:email_verify:"
const emailVerifyTTL = 24 * time.Hour

type IdentityRedisSessionAdapter struct {
	rdb go_redis.UniversalClient
}

func NewIdentityRedisSessionAdapter(rdb go_redis.UniversalClient) *IdentityRedisSessionAdapter {
	return &IdentityRedisSessionAdapter{rdb: rdb}
}

func (a *IdentityRedisSessionAdapter) IssueVerifyToken(ctx context.Context, userID int64) (string, error) {
	rawToken, err := cryptoutil.GenerateSecureToken()
	if err != nil {
		return "", fmt.Errorf("generate verify token: %w", err)
	}
	userIDStr := strconv.FormatInt(userID, 10)
	if err := a.rdb.Set(ctx, emailVerifyPrefix+rawToken, userIDStr, emailVerifyTTL).Err(); err != nil {
		return "", fmt.Errorf("store verify token: %w", err)
	}
	return rawToken, nil
}

func (a *IdentityRedisSessionAdapter) ParseVerifyToken(ctx context.Context, token string) (int64, error) {
	userIDStr, err := a.rdb.GetDel(ctx, emailVerifyPrefix+token).Result()
	if err != nil {
		if errors.Is(err, go_redis.Nil) {
			return 0, fmt.Errorf("verification token not found or already used")
		}
		return 0, fmt.Errorf("get verify token: %w", err)
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse user id from verify token: %w", err)
	}
	return userID, nil
}

func (a *IdentityRedisSessionAdapter) ParsePasswordResetToken(ctx context.Context, rawToken string) (int64, error) {
	hashedToken := cryptoutil.HashToken(rawToken)
	userIDStr, err := a.rdb.GetDel(ctx, resetPasswordPrefix+hashedToken).Result()
	if err != nil {
		if errors.Is(err, go_redis.Nil) {
			return 0, fmt.Errorf("reset token not found or expired")
		}
		return 0, fmt.Errorf("get reset token: %w", err)
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse user id from reset token: %w", err)
	}
	return userID, nil
}

func (a *IdentityRedisSessionAdapter) StoreSession(ctx context.Context, refreshTokenHash string, session *entity.Session, ttl time.Duration) error {
	b, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return a.rdb.Set(ctx, sessionPrefix+refreshTokenHash, b, ttl).Err()
}

// GetSession returns (nil, nil) on cache miss so callers can fall back to DB.
func (a *IdentityRedisSessionAdapter) GetSession(ctx context.Context, refreshTokenHash string) (*entity.Session, error) {
	data, err := a.rdb.Get(ctx, sessionPrefix+refreshTokenHash).Bytes()
	if err != nil {
		if errors.Is(err, go_redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session from cache: %w", err)
	}
	var session entity.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

func (a *IdentityRedisSessionAdapter) DeleteSession(ctx context.Context, refreshTokenHash string) error {
	return a.rdb.Del(ctx, sessionPrefix+refreshTokenHash).Err()
}

func (a *IdentityRedisSessionAdapter) DeleteSessions(ctx context.Context, refreshTokenHashes []string) error {
	if len(refreshTokenHashes) == 0 {
		return nil
	}
	keys := make([]string, len(refreshTokenHashes))
	for i, h := range refreshTokenHashes {
		keys[i] = sessionPrefix + h
	}
	_, err := a.rdb.Del(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("redis delete sessions failed: %w", err)
	}
	return nil
}

func (a *IdentityRedisSessionAdapter) StorePasswordResetToken(ctx context.Context, rawToken string, userID int64, ttl time.Duration) error {
	hashedToken := cryptoutil.HashToken(rawToken)
	userIDStr := strconv.FormatInt(userID, 10)
	return a.rdb.Set(ctx, resetPasswordPrefix+hashedToken, userIDStr, ttl).Err()
}
