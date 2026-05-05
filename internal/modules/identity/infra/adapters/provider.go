package adapters

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	platformAuth "github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/outbox"
	"github.com/google/wire"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ── Password Hasher ──────────────────────────────────────────────────────────
// BcryptHasher thỏa mãn ports.PasswordHasher trực tiếp qua structural typing.

func ProvideBcryptHasher(cfg *config.Config) ports.PasswordHasher {
	return platformAuth.NewBcryptHasher(cfg.JWT.BcryptCost)
}

// ── Token Manager ────────────────────────────────────────────────────────────
// JWTTokenManager trong platform/auth dùng auth.JWTClaims (platform type).
// jwtTokenManagerBridge là adapter mỏng cầu nối sang ports.AccessTokenClaims.

type jwtTokenManagerBridge struct {
	inner *platformAuth.JWTTokenManager
}

func (b *jwtTokenManagerBridge) GenerateAccessToken(ctx context.Context, c ports.AccessTokenClaims) (string, time.Time, error) {
	return b.inner.GenerateAccessToken(ctx, platformAuth.JWTClaims{
		UserID: c.UserID,
		Email:  c.Email,
		Role:   c.Role,
		Type:   c.Type,
	})
}

func (b *jwtTokenManagerBridge) GenerateRefreshToken(ctx context.Context, userID int64) (string, error) {
	return b.inner.GenerateRefreshToken(ctx, userID)
}

func ProvideJWTTokenManager(cfg *config.Config) ports.TokenManager {
	return &jwtTokenManagerBridge{inner: platformAuth.NewJWTTokenManager(cfg.JWT)}
}

// ── Verification Token ───────────────────────────────────────────────────────
// RedisVerificationTokenService thỏa mãn ports.VerificationTokenService trực tiếp.

func ProvideRedisVerificationTokenService(rdb *goredis.Client) ports.VerificationTokenService {
	return platformAuth.NewRedisVerificationTokenService(rdb)
}

// ── Event Publisher ──────────────────────────────────────────────────────────

func ProvideOutboxEventPublisher(recorder outbox.Recorder, log *zap.Logger) ports.EventPublisher {
	return NewOutboxEventPublisher(recorder, log)
}

// ── Clock ────────────────────────────────────────────────────────────────────

func ProvideRealClock() ports.Clock {
	return NewRealClock()
}

// ProviderSet gom tất cả port adapters của identity module.
// BcryptHasher, JWTTokenManager, RedisVerificationTokenService implement ở platform/auth.
// OutboxEventPublisher, RealClock là adapters identity-specific.
var ProviderSet = wire.NewSet(
	ProvideBcryptHasher,
	ProvideJWTTokenManager,
	ProvideRedisVerificationTokenService,
	ProvideOutboxEventPublisher,
	ProvideRealClock,
)
