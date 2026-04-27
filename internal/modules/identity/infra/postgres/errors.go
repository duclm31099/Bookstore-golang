package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Infrastructure không được để lỗi kỹ thuật chảy nguyên xi vào domain; blueprint gọi đây là anti-corruption boundary.
// Với Postgres, ít nhất bạn phải map pgx.ErrNoRows và 23505 unique_violation thành error có nghĩa cho module identity

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

//  repo có thể chuyển “không tìm thấy row” thành
// ErrUserNotFound, ErrSessionNotFound hoặc ErrCredentialNotFound
// tùy aggregate, thay vì ném pgx.ErrNoRows ra application layer
