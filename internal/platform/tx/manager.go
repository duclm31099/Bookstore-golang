package tx

import "context"

// Manager là interface cho phép application layer mở transaction
// mà không bị phụ thuộc trực tiếp vào *pgxpool.Pool.
// Dùng interface này để dễ mock trong unit test.
type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
