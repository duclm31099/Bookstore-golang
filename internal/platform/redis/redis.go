package redis

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

/*
Anti-pattern cần tránh

	Đặt key business của auth, reader, download, inventory hết vào platform/redis.

	Dùng Redis làm source of truth cho payment, entitlement, inventory canonical.

	Dùng Redis mà không đặt timeout.

	Cache xong quên invalidation strategy.

# Cách nghĩ đúng

Redis giống như “bàn phụ” cạnh quầy chính: lấy đồ nhanh, xử lý nhanh, nhưng sổ cái thật vẫn nằm ở PostgreSQL.
Nếu bạn coi bàn phụ là két sắt chính, sớm muộn sẽ thất lạc dữ liệu.
*/
func NewRedisClient(cfg config.RedisConfig) (*goredis.Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}

func Ping(ctx context.Context, rdb *goredis.Client) error {
	return rdb.Ping(ctx).Err()
}

func Key(parts ...string) string {
	res := ""
	for i, p := range parts {
		if i > 0 {
			res += ":"
		}
		res += p
	}
	return res
}
