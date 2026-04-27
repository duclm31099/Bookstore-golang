package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository cần một abstraction tối thiểu mà cả *pgxpool.Pool và pgx.Tx đều có thể thỏa mãn,
// vì blueprint yêu cầu repository chạy được trong cùng transaction context do application quản lý.

type Executor interface {
	Exec(ctx context.Context, sql string, argument ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, argument ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, argument ...any) pgx.Row
}

// nếu repo phụ thuộc trực tiếp vào *pgxpool.Pool,
// bạn sẽ rất khó tái sử dụng cùng method bên trong transaction;
// còn nếu phụ thuộc vào pgx.Tx thì lại không dùng được ngoài transaction.
// Executor là điểm cân bằng đúng kiểu backend pragmatism.
