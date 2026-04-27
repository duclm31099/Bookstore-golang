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

type SessionRepository struct {
	BaseRepository
}

func NewSessionRepository(pool *pgxpool.Pool) domain.SessionRepository {
	return &SessionRepository{
		BaseRepository: NewBaseRepository(pool),
	}
}

const (
	queryInsertSession = `
		INSERT INTO user_sessions (user_id, device_id, refresh_token_hash, session_status, expires_at, ip_address, user_agent, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	queryGetByRefreshTokenHash = `
		SELECT id, user_id, device_id, refresh_token_hash, session_status, expires_at, ip_address, user_agent, last_seen_at, revoked_at, revoked_reason, created_at, updated_at
		FROM user_sessions
		WHERE refresh_token_hash = $1
	`

	queryListActiveByUserID = `
		SELECT id, user_id, device_id, refresh_token_hash, session_status, expires_at, ip_address, user_agent, last_seen_at, revoked_at, revoked_reason, created_at, updated_at
		FROM user_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
		ORDER BY created_at DESC`

	queryRevokeSession = `
		UPDATE user_sessions 
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE id = $1 AND revoked_at IS NULL AND expires_at > $3
	`

	queryRevokeAllByUserID = `
		UPDATE user_sessions 
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $3
	`

	queryUpdateSession = `
		UPDATE user_sessions
		SET refresh_token_hash = $2, expires_at = $3, last_seen_at = $4
		WHERE id = $1
	`
)

func (r *SessionRepository) Update(ctx context.Context, session *entity.Session) error {
	executor := tx.GetExecutor(ctx, r.pool)

	_, err := executor.Exec(ctx, queryUpdateSession, session.ID, session.RefreshTokenHash, session.ExpiredAt, session.LastSeenAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *SessionRepository) Insert(ctx context.Context, session *entity.Session) error {
	executor := tx.GetExecutor(ctx, r.pool)

	err := executor.QueryRow(ctx, queryInsertSession,
		session.UserID, session.DeviceID, session.RefreshTokenHash, session.SessionStatus,
		session.ExpiredAt, session.IPAddress, session.UserAgent, session.LastSeenAt,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)

	return err
}

func (r *SessionRepository) GetByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error) {
	return r.getByRefreshTokenHash(ctx, hash, false)
}

func (r *SessionRepository) ListActiveByUserID(ctx context.Context, userID int64) ([]*entity.Session, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListActiveByUserID, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*entity.Session
	for rows.Next() {
		row, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, mapSessionRowToEntity(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, id int64, revokedAt time.Time) error {
	executor := tx.GetExecutor(ctx, r.pool)

	result, err := executor.Exec(ctx, queryRevokeSession, id, revokedAt, time.Now())
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return err_domain.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) GetByRefreshTokenHashForUpdate(ctx context.Context, hash string) (*entity.Session, error) {
	return r.getByRefreshTokenHash(ctx, hash, true)
}

func (r *SessionRepository) RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error {
	executor := tx.GetExecutor(ctx, r.pool)

	_, err := executor.Exec(ctx, queryRevokeAllByUserID, userID, revokedAt, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (r *SessionRepository) getByRefreshTokenHash(ctx context.Context, hash string, forUpdate bool) (*entity.Session, error) {
	exec := tx.GetExecutor(ctx, r.pool)
	sql := queryGetByRefreshTokenHash
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	row, err := scanSession(exec.QueryRow(ctx, sql, hash))
	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrSessionNotFound
		}
		return nil, err
	}
	return mapSessionRowToEntity(row), nil
}
