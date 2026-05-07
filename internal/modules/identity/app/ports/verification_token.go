package ports

import "context"

type VerificationTokenService interface {
	IssueEmailVerificationToken(ctx context.Context, userID int64) (string, error)
	ParseEmailVerificationToken(ctx context.Context, token string) (int64, error)
}

type RedisSessionService interface {
	DeleteSession(ctx context.Context, key string) error
	SetUserSession(ctx context.Context, key string, value any, TTL int64) error
	GetUserSession(ctx context.Context, key string) (any, error)
}

const RedisSessionKeyPrefix = "refresh_token:"
