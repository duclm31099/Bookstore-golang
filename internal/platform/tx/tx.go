package tx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Transaction boundary hoàn toàn thuộc về Application layer
Vì vậy platform/tx phải cung cấp cơ chế chuẩn để mở transaction một lần ở service,
rồi repository bên dưới tự lấy pgx.Tx từ context nếu có
*/
type txKey struct{}

type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

func (m *Manager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	txx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	ctx = inject(ctx, txx)

	if err := fn(ctx); err != nil {
		_ = txx.Rollback(ctx)
		return err
	}

	if err := txx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func Extract(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
func inject(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}
