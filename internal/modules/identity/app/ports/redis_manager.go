package ports

import "context"

type RedisSessionService interface {
	IssueVerifyToken(ctx context.Context, userID int64) (string, error)
	ParseVerifyToken(ctx context.Context, token string) (int64, error)
	DeleteSession(ctx context.Context, key string) error
	SetUserSession(ctx context.Context, key string, value any, TTL int64) error
	GetUserSession(ctx context.Context, key string) (any, error)
}

const RedisSessionKeyPrefix = "refresh_token:"
