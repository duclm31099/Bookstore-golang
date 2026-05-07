package entity

import (
	"time"

	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
)

type Device struct {
	ID            int64
	UserID        int64
	Fingerprint   string
	Label         string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	RevokedAt     *time.Time
	RevokedReason *string
	Metadata      map[string]interface{}
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (d *Device) IsRevoked() bool {
	return d.RevokedAt != nil
}

// Revoke đánh dấu device bị thu hồi quyền
// Sau khi revoke, session từ device này không còn hợp lệ
func (d *Device) Revoke(now time.Time) error {
	if d.IsRevoked() {
		return nil
	}
	d.RevokedAt = &now
	return nil
}

// AssertOwnership kiểm tra device có thuộc về user không
// Dùng trước mọi mutation để enforce ownership boundary
func (d *Device) AssertOwnership(userID int64) error {
	if d.UserID != userID {
		return err.ErrDeviceNotOwned
	}
	return nil
}

// UpdateLastSeen cập nhật thời gian hoạt động gần nhất
func (d *Device) UpdateLastSeen(now time.Time) {
	d.LastSeenAt = now
}
