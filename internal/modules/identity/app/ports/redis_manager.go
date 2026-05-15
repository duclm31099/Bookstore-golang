package ports

import (
	"context"
	"time"
)

type RedisSessionService interface {
	// IssueVerifyToken phát hành một mã token xác thực (verify token) cho user ID.
	// Hàm này thường được sử dụng trong quy trình verify email hoặc reset password.
	IssueVerifyToken(ctx context.Context, userID int64) (string, error)
	// ParseVerifyToken phân tích một mã token xác thực và trả về user ID.
	ParseVerifyToken(ctx context.Context, token string) (int64, error)
	// Xoá session khi user logout, thay đổi password
	DeleteSession(ctx context.Context, key string) error
	// lưu user session (key: refresh_token:device_fingerprint, value: user session struct)
	SetUserSession(ctx context.Context, key string, value any, TTL int64) error
	// lưu user session
	GetUserSession(ctx context.Context, key string) (any, error)
	// Delete multiple sessions by keys
	DeleteMultipleSessions(ctx context.Context, keys []string) error
}

const RedisSessionKeyPrefix = "refresh_token:"

// BlacklistPort là interface quy định các hành vi cần có để quản lý danh sách đen token
type BlacklistPort interface {
	// AddToBlacklist thêm một token ID vào danh sách đen với thời gian sống (TTL) cụ thể
	AddToBlacklist(ctx context.Context, jti string, expiration time.Duration) error

	// IsTokenBlacklisted kiểm tra xem một token ID có nằm trong danh sách đen không
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
}
