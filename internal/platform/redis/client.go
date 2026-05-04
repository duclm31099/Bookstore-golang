package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type Client interface {
	goredis.Cmdable
	Close() error
	Ping(ctx context.Context) *goredis.StatusCmd
}

func NewClient(cfg Config) Client {
	cfg = cfg.withDefaults()
	return goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
}
