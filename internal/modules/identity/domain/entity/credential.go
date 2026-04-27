// internal/modules/identity/domain/entity/credential.go
package entity

import (
	"time"
)

// Credential lưu thông tin xác thực của user — tách biệt hoàn toàn
// khỏi User entity để:
//  1. Không bao giờ bị accidental serialize ra response
//  2. Chỉ login/reset-password flow mới có CredentialRepository dependency
//  3. Lifecycle khác User: chỉ thay đổi khi đổi/reset password
//  4. Có thể mở rộng về sau: thêm OTP, passkey, OAuth credentials
//     mà không làm phình User aggregate
//
// KHÔNG bao giờ log, marshal, hoặc trả Credential ra bên ngoài domain.
// Sau khi verify xong, caller chỉ nhận bool hoặc error, không nhận Credential.
type Credential struct {
	UserID            int64
	PasswordHash      string    // bcrypt/argon2id hash — NEVER log
	PasswordAlgo      string
	PasswordChangedAt time.Time // dùng để invalidate sessions cũ sau đổi password
	FailedLoginCount  int
	LastFailedLoginAt *time.Time
}

// IsPasswordChangeRequired kiểm tra xem password có cần đổi không
// Ví dụ: admin force-reset hoặc password quá cũ theo policy
// Phase 1 chưa enforce nhưng nên chuẩn bị field từ đầu
func (c *Credential) IsPasswordChangeRequired(now time.Time, maxAgeDays int) bool {
	if maxAgeDays <= 0 {
		return false // policy disabled
	}
	age := now.Sub(c.PasswordChangedAt)
	return age.Hours() > float64(maxAgeDays)*24
}

// MarkPasswordChanged cập nhật timestamp sau khi đổi password thành công
// Application service gọi method này sau khi đã hash password mới
// và persist thành công
func (c *Credential) MarkPasswordChanged(newHash string, algo string, now time.Time) {
	c.PasswordHash = newHash
	c.PasswordAlgo = algo
	c.PasswordChangedAt = now
}
