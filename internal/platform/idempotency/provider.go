package idempotency

import (
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func NewClock() Clock {
	return systemClock{}
}

func NewDefaultConfig() Config {
	return Config{
		InProgressTTL: 5 * time.Minute,
		CompletedTTL:  24 * time.Hour,
	}
}

func NewDefaultService(
	store Store,
	eventStore ProcessedEventStore,
	clock Clock,
	cfg Config,
) Service {
	return NewService(store, eventStore, clock, cfg)
}

func ProvideRedisStore(rdb redis.UniversalClient) Store {
	return NewRedisStore(rdb)
}

func ProvideProcessedEventStore(db DBTX) ProcessedEventStore {
	return NewProcessedEventStore(db)
}

var ProviderSet = wire.NewSet(
	NewClock,
	NewDefaultConfig,
	ProvideRedisStore,
	ProvideProcessedEventStore,
	NewDefaultService,
)
