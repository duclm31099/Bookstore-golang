package postgres

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	err_domain "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	object "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	BaseRepository
}

func NewUserRepository(pool *pgxpool.Pool) domain.UserRepository {
	return &UserRepository{BaseRepository: NewBaseRepository(pool)}
}

const (
	queryGetUserByID = `
		SELECT id, email, full_name, phone, user_type, account_status, email_verified_at, last_login_at, locked_reason, metadata, version, created_at, updated_at
		FROM users WHERE id = $1
	`
	queryGetUserByEmail = `
		SELECT id, email, full_name, phone, user_type, account_status, email_verified_at, last_login_at, locked_reason, metadata, version, created_at, updated_at
		FROM users WHERE email = $1
	`
	queryInsertUser = `
		INSERT INTO users (email, full_name, phone, user_type, account_status, email_verified_at, metadata, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`
	queryUpdateStatus = `
		UPDATE users 
		SET account_status = $1, updated_at = NOW(), version = version + 1
		WHERE id = $2
	`
	queryMarkEmailVerified = `
		UPDATE users
		SET 
			email_verified_at = $1, 
			account_status = 'active',
			version = version + 1
		WHERE 
			id = $2 
			AND email_verified_at IS NULL;
	`

	queryCheckExistEmail = `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
)

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	exec := tx.GetExecutor(ctx, r.pool)
	pgxRow := exec.QueryRow(ctx, queryGetUserByID, id)
	user, err := scanUser(pgxRow)

	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrUserNotFound
		}
		return nil, err
	}

	return mapUserRowToEntity(user)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email object.Email) (*entity.User, error) {
	exec := tx.GetExecutor(ctx, r.pool)

	pgxRow := exec.QueryRow(ctx, queryGetUserByEmail, email.String())
	user, err := scanUser(pgxRow)

	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrUserNotFound
		}
		return nil, err
	}

	return mapUserRowToEntity(user)
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email object.Email) (bool, error) {
	exec := tx.GetExecutor(ctx, r.pool)

	var exists bool
	err := exec.QueryRow(ctx, queryCheckExistEmail, email.String()).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *UserRepository) Insert(ctx context.Context, user *entity.User) error {
	exec := tx.GetExecutor(ctx, r.pool)

	err := exec.QueryRow(ctx, queryInsertUser,
		user.Email.String(),
		user.FullName,
		user.Phone,
		user.UserType,
		user.Status,
		user.EmailVerifiedAt,
		user.Metadata,
		user.Version,
	).Scan(&user.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return err_domain.ErrEmailAlreadyExist
		}
		return err
	}

	return nil
}

func (r *UserRepository) UpdateStatus(ctx context.Context, id int64, status object.UserStatus) error {
	exec := tx.GetExecutor(ctx, r.pool)
	cmdTag, err := exec.Exec(ctx, queryUpdateStatus, status, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return err_domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id int64, verifiedAt time.Time) error {
	exec := tx.GetExecutor(ctx, r.pool)
	cmdTag, err := exec.Exec(ctx, queryMarkEmailVerified, verifiedAt, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return err_domain.ErrUserNotFound
	}
	return nil
}
