package postgres

import (
	"context"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/query"
	err_domain "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QueryRepository struct {
	BaseRepository
}

func NewQueryRepository(pool *pgxpool.Pool) query.QueryRepository {
	return &QueryRepository{BaseRepository: NewBaseRepository(pool)}
}

const (
	// scanUser expects: id, email, full_name, phone, user_type, account_status,
	// email_verified_at, last_login_at, locked_reason, metadata, version, created_at, updated_at
	queryMe = `
		SELECT id, email, full_name, phone, user_type, account_status,
			email_verified_at, last_login_at, locked_reason, metadata,
			version, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	// scanSession expects: id, user_id, device_id, refresh_token_hash, session_status,
	// expires_at, ip_address, user_agent, last_seen_at, revoked_at, revoked_reason, created_at, updated_at
	queryListSession = `
		SELECT id, user_id, device_id, refresh_token_hash, session_status,
			expires_at, ip_address, user_agent, last_seen_at,
			revoked_at, revoked_reason, created_at, updated_at
		FROM user_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	// scanDevice expects: id, user_id, fingerprint_hash, device_label, first_seen_at,
	// last_seen_at, revoked_at, revoked_reason, metadata, created_at, updated_at
	queryListDevice = `
		SELECT id, user_id, fingerprint_hash, device_label, first_seen_at,
			last_seen_at, revoked_at, revoked_reason, metadata, created_at, updated_at
		FROM user_devices
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	// scanAddress expects: id, user_id, recipient_name, recipient_phone,
	// address_line1, address_line2, province_code, district_code, ward_code,
	// postal_code, country_code, is_default, version, created_at, updated_at
	queryListAddress = `
		SELECT id, user_id, recipient_name, recipient_phone,
			address_line1, address_line2, province_code, district_code, ward_code,
			postal_code, country_code, is_default, version, created_at, updated_at
		FROM addresses
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`
)

func (r *QueryRepository) GetMe(ctx context.Context, userID int64) (*query.MeView, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	row := executor.QueryRow(ctx, queryMe, userID)
	user, err := scanUser(row)

	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrUserNotFound
		}
		return nil, err
	}
	return mapUserRowToMeView(user), nil
}

func (r *QueryRepository) ListSessions(ctx context.Context, userID int64) ([]*query.SessionView, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListSession, userID)
	defer rows.Close()

	if err != nil {
		return nil, err
	}
	var sessions []*query.SessionView
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, mapSessionRowToView(s))
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return sessions, nil
}

func (r *QueryRepository) ListDevices(ctx context.Context, userID int64) ([]*query.DeviceView, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListDevice, userID)
	defer rows.Close()

	if err != nil {
		return nil, err
	}

	var devices []*query.DeviceView
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, mapDeviceRowToView(d))
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return devices, nil
}

func (r *QueryRepository) ListAddresses(ctx context.Context, userID int64) ([]*query.AddressView, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListAddress, userID)
	defer rows.Close()

	if err != nil {
		return nil, err
	}

	var addresses []*query.AddressView
	for rows.Next() {
		a, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, mapAddressRowToView(a))
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return addresses, nil
}
