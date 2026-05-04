package redis

import (
	"context"
	"time"
)

type RateLimiter struct {
	rdb Client
}

type RateLimitResult struct {
	Allowed   bool
	Count     int64
	Remaining int64
	ResetIn   time.Duration
}

func NewRateLimiter(rdb Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

func (r *RateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (RateLimitResult, error) {
	n, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return RateLimitResult{}, ErrUnavailable
	}
	if n == 1 {
		if err := r.rdb.Expire(ctx, key, window).Err(); err != nil {
			return RateLimitResult{}, ErrUnavailable
		}
	}

	ttl, err := r.rdb.TTL(ctx, key).Result()
	if err != nil {
		return RateLimitResult{}, ErrUnavailable
	}

	remaining := limit - n
	if remaining < 0 {
		remaining = 0
	}

	return RateLimitResult{
		Allowed:   n <= limit,
		Count:     n,
		Remaining: remaining,
		ResetIn:   ttl,
	}, nil
}
