package entity

import (
	"time"

	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	valueobject "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
)

type User struct {
	ID              int64
	Email           valueobject.Email
	FullName        string
	Phone           *string
	UserType        string
	Status          valueobject.UserStatus
	EmailVerifiedAt *time.Time
	LastLoginAt     *time.Time
	LockedReason    *string
	Metadata        map[string]interface{}
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsEmailVerified trả lời câu hỏi business: "email này đã được xác minh chưa?"
func (u *User) IsEmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

// CanPerformDigitalActions enforce rule từ SRD:
// user chưa verify email không được download hoặc làm digital actions
func (u *User) CanPerformDigitalActions() error {
	if u.IsEmailVerified() == false {
		return err.ErrEmailNotVerified
	}

	return nil

}

// MarkEmailVerified là cách duy nhất để verify email trong domain
// Application service gọi method này sau khi validate token xong
func (u *User) MarkEmailVerified(now time.Time) error {
	if u.IsEmailVerified() {
		return nil // idempotent — verify lại thì không lỗi
	}

	t := now
	u.EmailVerifiedAt = &t
	u.Status = valueobject.UserStatusActive
	u.UpdatedAt = now
	return nil
}

// Suspend chuyển user sang suspended nếu policy cho phép
func (u *User) Suspend() error {

	if err := u.Status.CanTransitionTo(valueobject.UserStatusSuspended); err != nil {
		return err
	}

	u.Status = valueobject.UserStatusSuspended
	u.UpdatedAt = time.Now()
	return nil

}

// Ban chuyển user sang banned — terminal state
func (u *User) Ban() error {
	if err := u.Status.CanTransitionTo(valueobject.UserStatusBanned); err != nil {
		return err
	}

	u.Status = valueobject.UserStatusBanned
	u.UpdatedAt = time.Now()
	return nil
}
