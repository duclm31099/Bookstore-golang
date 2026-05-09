package adapters

import (
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	platformAuth "github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/outbox"
	"github.com/google/wire"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ── Token Manager From Plaform/Auth ────────────────────────────────────────────────────────────
func ProvideJWTAuthManager(cfg *config.Config) ports.AuthManager {
	return platformAuth.NewAuthManager(cfg.JWT)
}

// ── Session Service ──────────────────────────────────────────────────────────
// RedisSessionService thỏa mãn ports.RedisSessionService trực tiếp.

func ProvideRedisSessionService(rdb *goredis.Client) ports.RedisSessionService {
	return platformAuth.NewRedisSessionService(rdb)
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
	ProvideJWTAuthManager,
	ProvideRedisSessionService,
	ProvideOutboxEventPublisher,
	ProvideRealClock,
)
