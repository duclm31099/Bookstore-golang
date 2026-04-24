package db

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Anti-pattern cần tránh

	Tạo nhiều pool tùy tiện.
	Tự Open DB ở từng module.
	Đặt business query ở platform/db.
	Giữ transaction quá lâu rồi gọi network ra ngoài.
	Quăng pgxpool.Pool lung tung mà không rõ lifecycle.

Cách nghĩ đúng

	Pool DB là “bãi đỗ xe” đắt tiền. Nếu handler mở transaction rồi đứng chờ webhook, mail hay payment provider, bạn đang cho xe chiếm chỗ mà không di chuyển gì cả; đến lúc traffic cao là tắc ngay
*/
func NewPool(cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, err
	}

	poolCfg.MaxConns = cfg.MaxOpenConns
	poolCfg.MinConns = cfg.MinIdleConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthcheckPeriod

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}
