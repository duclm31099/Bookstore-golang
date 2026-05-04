package redis

import (
	"context"
	"time"
)

type QuotaStore struct {
	rdb Client
}

func NewQuotaStore(rdb Client) *QuotaStore {
	return &QuotaStore{rdb: rdb}
}

func (q *QuotaStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := q.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, ErrUnavailable
	}
	if n == 1 && ttl > 0 {
		if err := q.rdb.Expire(ctx, key, ttl).Err(); err != nil {
			return 0, ErrUnavailable
		}
	}
	return n, nil
}

func (q *QuotaStore) Get(ctx context.Context, key string) (int64, error) {
	n, err := q.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0, ErrUnavailable
	}
	return n, nil
}
