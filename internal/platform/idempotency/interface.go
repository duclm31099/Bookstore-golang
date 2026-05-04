package idempotency

import (
	"context"
	"time"
)

type Clock interface {
	Now() time.Time
}

type Store interface {
	Reserve(ctx context.Context, rec Record, ttl time.Duration) (ReserveDecision, *Record, error)
	Get(ctx context.Context, key string) (*Record, error)
	Complete(ctx context.Context, key string, result Result, ttl time.Duration, now time.Time) error
	Release(ctx context.Context, key string) error
}

type ProcessedEventStore interface {
	TryMarkProcessed(
		ctx context.Context,
		consumerName string,
		eventID string,
		processedAt time.Time,
		metadata map[string]any,
	) (bool, error)
}

type Service interface {
	BeginHTTP(ctx context.Context, scope, rawKey, requestHash string) (BeginResult, error)
	CompleteHTTP(ctx context.Context, scope, rawKey string, result Result) error
	ReleaseHTTP(ctx context.Context, scope, rawKey string) error
	GetHTTP(ctx context.Context, scope, rawKey string) (*Record, error)

	TryProcessEvent(ctx context.Context, consumerName, eventID string, metadata map[string]any) (bool, error)
}
