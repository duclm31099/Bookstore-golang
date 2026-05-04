package redis

import "github.com/google/wire"

func ProvideKeyBuilder(cfg Config) KeyBuilder {
	cfg = cfg.withDefaults()
	return NewKeyBuilder(cfg.KeyPrefix)
}

var ProviderSet = wire.NewSet(
	NewClient,
	NewCache,
	NewLocker,
	NewRateLimiter,
	NewQuotaStore,
	NewHealthChecker,
	ProvideKeyBuilder,
)
