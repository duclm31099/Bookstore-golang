package entity

import (
	"time"

	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
)

// Session đại diện cho một refresh token lifecycle
// Access token là ephemeral và stateless (JWT), không được lưu
// Refresh token được lưu dưới dạng hash để không expose raw value
type Session struct {
	ID               int64
	UserID           int64
	RefreshTokenHash string // NEVER store raw refresh token
	DeviceID         *int64 // nil nếu device chưa đăng ký
	SessionStatus    string
	ExpiredAt        time.Time
	IPAddress        string
	UserAgent        string
	LastSeenAt       time.Time
	RevokedAt        *time.Time
	RevokedReason    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsRevoked kiểm tra session đã bị thu hồi chưa
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsExpired kiểm tra session đã hết hạn chưa
func (s *Session) IsExpired(now time.Time) bool {
	//  Check now is after expired at
	return now.After(s.ExpiredAt)
}

// Revoke thu hồi session
func (s *Session) Revoke(now time.Time) {
	if s.IsRevoked() {
		return // idempotent
	}
	s.RevokedAt = &now
}

// IsUsable tổng hợp kiểm tra session có dùng được không
// Đây là method quan trọng nhất — dùng trong RefreshSession flow
func (s *Session) isUseable(now time.Time) error {
	if s.IsRevoked() {
		return err.ErrSessionRevoked
	}

	if s.IsExpired(now) {
		return err.ErrSessionExpired
	}

	return nil
}

// Rotate dùng trong trường hợp single-rotate session
func (s *Session) Rotate(newRefreshTokenHash string, now time.Time) {
	s.RefreshTokenHash = newRefreshTokenHash
	s.ExpiredAt = now.Add(30 * 24 * time.Hour)
}
