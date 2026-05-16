package ports

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
)

type RedisSessionService interface {
	// IssueVerifyToken phát hành một mã token xác thực (verify token) cho user ID.
	IssueVerifyToken(ctx context.Context, userID int64) (string, error)
	// ParseVerifyToken phân tích một mã token xác thực và trả về user ID.
	ParseVerifyToken(ctx context.Context, token string) (int64, error)
	// ParsePasswordResetToken xác minh token đặt lại mật khẩu, trả về userID.
	// Atomic GetDel: token bị xóa ngay khi đọc — không thể dùng lại (single-use).
	ParsePasswordResetToken(ctx context.Context, rawToken string) (int64, error)
	// StorePasswordResetToken lưu password reset token vào Redis với TTL.
	// rawToken được hash trước khi lưu — Redis không chứa raw token.
	StorePasswordResetToken(ctx context.Context, rawToken string, userID int64, ttl time.Duration) error

	// StoreSession lưu session vào Redis cache với TTL.
	StoreSession(ctx context.Context, refreshTokenHash string, session *entity.Session, ttl time.Duration) error
	// GetSession lấy session từ Redis cache. Trả về (nil, nil) nếu cache miss.
	GetSession(ctx context.Context, refreshTokenHash string) (*entity.Session, error)
	// DeleteSession xóa một session khỏi Redis cache.
	DeleteSession(ctx context.Context, refreshTokenHash string) error
	// DeleteSessions xóa nhiều session cùng lúc (VD: logout tất cả thiết bị).
	DeleteSessions(ctx context.Context, refreshTokenHashes []string) error
}

// BlacklistPort là interface quy định các hành vi cần có để quản lý danh sách đen token
type BlacklistPort interface {
	// AddToBlacklist thêm một token ID vào danh sách đen với thời gian sống (TTL) cụ thể
	AddToBlacklist(ctx context.Context, jti string, expiration time.Duration) error

	// IsTokenBlacklisted kiểm tra xem một token ID có nằm trong danh sách đen không
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
}
