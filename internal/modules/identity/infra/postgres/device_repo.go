package postgres

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeviceRepository struct {
	BaseRepository
}

func NewDeviceRepository(pool *pgxpool.Pool) domain.DeviceRepository {
	return &DeviceRepository{
		BaseRepository: NewBaseRepository(pool),
	}
}

const (
	queryGetDeviceByID = `
		SELECT id, user_id, fingerprint_hash, device_label, first_seen_at, last_seen_at, revoked_at, revoked_reason, metadata, created_at, updated_at
		FROM user_devices
		WHERE id = $1
	`

	queryGetDeviceByFingerprint = `
		SELECT id, user_id, fingerprint_hash, device_label, first_seen_at, last_seen_at, revoked_at, revoked_reason, metadata, created_at, updated_at
		FROM user_devices
		WHERE user_id = $1 AND fingerprint_hash = $2
	`

	queryListActiveDeviceByUserID = `
		SELECT id, user_id, fingerprint_hash, device_label, first_seen_at, last_seen_at, revoked_at, revoked_reason, metadata, created_at, updated_at
		FROM user_devices
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY last_seen_at DESC
	`

	queryUpsertDevice = `
		INSERT INTO user_devices (user_id, fingerprint_hash, device_label, first_seen_at, last_seen_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, fingerprint_hash) DO UPDATE SET
			device_label = EXCLUDED.device_label,
			last_seen_at = EXCLUDED.last_seen_at,
			revoked_at = NULL
		RETURNING id, first_seen_at
	`

	queryRevokeDevice = `
		UPDATE user_devices
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE id = $1
	`
)

func (r *DeviceRepository) GetByID(ctx context.Context, id int64) (*entity.Device, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	row := executor.QueryRow(ctx, queryGetDeviceByID, id)

	deviceRow, err := scanDevice(row)
	if err != nil {
		return nil, err
	}

	return mapDeviceRowToEntity(deviceRow), nil
}

func (r *DeviceRepository) GetByFingerprint(ctx context.Context, userID int64, fingerprint string) (*entity.Device, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	row := executor.QueryRow(ctx, queryGetDeviceByFingerprint, userID, fingerprint)
	deviceRow, err := scanDevice(row)

	if err != nil {
		return nil, err
	}

	return mapDeviceRowToEntity(deviceRow), nil
}
func (r *DeviceRepository) ListActiveByUserID(ctx context.Context, userID int64) ([]*entity.Device, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListActiveDeviceByUserID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*entity.Device
	for rows.Next() {
		deviceRow, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, mapDeviceRowToEntity(deviceRow))
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return devices, nil
}
func (r *DeviceRepository) Upsert(ctx context.Context, device *entity.Device) error {
	executor := tx.GetExecutor(ctx, r.pool)

	err := executor.QueryRow(ctx, queryUpsertDevice, device.UserID,
		device.Fingerprint, device.Label,
		device.FirstSeenAt, device.LastSeenAt, device.RevokedAt).Scan(&device.ID, &device.FirstSeenAt)
	if err != nil {
		return err
	}
	return nil
}
func (r *DeviceRepository) Revoke(ctx context.Context, id int64, revokedAt time.Time) error {
	executor := tx.GetExecutor(ctx, r.pool)

	_, err := executor.Exec(ctx, queryRevokeDevice, id, revokedAt)
	if err != nil {
		return err
	}
	return nil
}
