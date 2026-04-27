package valueobject

import err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"

type UserStatus string

const (
	UserStatusActive              UserStatus = "active"               // đã verify, hoạt động bình thường
	UserStatusBanned              UserStatus = "locked"               // cấm vĩnh viễn
	UserStatusSuspended           UserStatus = "disabled"             // khoá tạm thời
	UserStatusPendingVerification UserStatus = "pending_verification" // chưa verify email
)

// IsActive check user có active không
func (s UserStatus) IsActive() bool {
	return s == UserStatusActive
}

// CanLogin kiểm tra user có thể đăng nhập không
func (s UserStatus) CanLogin() bool {
	return s == UserStatusActive || s == UserStatusPendingVerification
}

// Transitions mô tả các chuyển trạng thái hợp lệ
// Đây là state machine nhỏ của User lifecycle
func (s UserStatus) CanTransitionTo(next UserStatus) error {
	allowed := map[UserStatus][]UserStatus{
		UserStatusPendingVerification: {UserStatusActive},
		UserStatusActive:              {UserStatusSuspended, UserStatusBanned},
		UserStatusSuspended:           {UserStatusActive, UserStatusBanned},
		UserStatusBanned:              {},
	}

	for _, a := range allowed[s] {
		if a == next {
			return nil
		}
	}

	return err.New("IDENTITY_INVALID_STATUS_TRANSITION", "cannot transition from "+string(s)+" to "+string(next))
}
