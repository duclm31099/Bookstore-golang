package postgres

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	err_domain "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CredentialRepository struct {
	BaseRepository
}

func NewCredentialRepository(pool *pgxpool.Pool) domain.CredentialRepository {
	return &CredentialRepository{BaseRepository: NewBaseRepository(pool)}
}

const (
	queryCredentialGetByUserID = `
		SELECT user_id, password_hash, password_algo, password_changed_at, failed_login_count, last_failed_login_at
		FROM user_credentials
		WHERE user_id = $1
	`
	queryInsert = `
		INSERT INTO user_credentials (user_id, password_hash, password_algo, password_changed_at, failed_login_count, last_failed_login_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	queryUpdatePasswordHash = `
		UPDATE user_credentials
		SET password_hash = $2, password_changed_at = $3
		WHERE user_id = $1
	`
)

func (r *CredentialRepository) GetByUserID(ctx context.Context, userID int64) (*entity.Credential, error) {
	exec := tx.GetExecutor(ctx, r.pool)

	row := exec.QueryRow(ctx, queryCredentialGetByUserID, userID)

	credRow, err := scanCredential(row)
	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrCredentialNotFound
		}
		return nil, err
	}

	return mapCredentialRowToEntity(credRow), nil
}

func (r *CredentialRepository) Insert(ctx context.Context, cred *entity.Credential) error {
	exec := tx.GetExecutor(ctx, r.pool)

	_, err := exec.Exec(ctx, queryInsert, cred.UserID, cred.PasswordHash, cred.PasswordAlgo, cred.PasswordChangedAt, cred.FailedLoginCount, cred.LastFailedLoginAt)

	return err
}

func (r *CredentialRepository) UpdatePasswordHash(ctx context.Context, userID int64, hash string, changedAt time.Time) error {
	exec := tx.GetExecutor(ctx, r.pool)

	tag, err := exec.Exec(ctx, queryUpdatePasswordHash, userID, hash, changedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return err_domain.ErrCredentialNotFound
	}
	return nil
}
