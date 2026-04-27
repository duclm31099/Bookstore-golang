package ports

import (
	"context"
	"time"
)

type AccessTokenClaims struct {
	UserID int64
	Email  string
	Role   string
	Type   string
}

type TokenManager interface {
	GenerateAccessToken(ctx context.Context, claims AccessTokenClaims) (token string, expiresAt time.Time, err error)
	GenerateRefreshToken(ctx context.Context, userID int64) (raw string, err error)
}
