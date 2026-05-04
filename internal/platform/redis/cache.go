package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb Client
}

func NewCache(rdb Client) *Cache {
	return &Cache{rdb: rdb}
}

func (c *Cache) Get(ctx context.Context, key string, out any) error {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return ErrCacheMiss
		}
		return ErrUnavailable
	}
	if err := Unmarshal(b, out); err != nil {
		return err
	}
	return nil
}

func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := Marshal(value)
	if err != nil {
		return err
	}
	if err := c.rdb.Set(ctx, key, b, ttl).Err(); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return ErrUnavailable
	}
	return nil
}
