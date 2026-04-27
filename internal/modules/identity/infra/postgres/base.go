package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// BaseRepository là nơi chứa dependency tối thiểu dùng lại cho mọi repo implementation.
// Điều này giúp constructors nhất quán và tránh copy/paste pool handling khắp nơi.
type BaseRepository struct {
	pool *pgxpool.Pool
}

func NewBaseRepository(pool *pgxpool.Pool) BaseRepository {
	return BaseRepository{pool: pool}
}
