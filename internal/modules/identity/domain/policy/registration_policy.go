package policy

import (
	"unicode"

	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	valueobject "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
)

// RegistrationPolicy tập trung toàn bộ rule cho đăng ký tài khoản
// Tách ra để: testable độc lập, dễ thay đổi rule, không lẫn vào service
type RegisterPolicy struct {
	MinPassLength int
	MaxPassLength int
}

func NewRegisterPolicy() *RegisterPolicy {
	return &RegisterPolicy{
		MinPassLength: 8,
		MaxPassLength: 72,
	}
}

// ValidatePassword kiểm tra password strength
// SRD yêu cầu password policy production-grade
func (p *RegisterPolicy) ValidatePassword(password string) error {
	length := len(password)
	if length < p.MinPassLength || length > p.MaxPassLength {
		return err.ErrPasswordTooWeak
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return err.ErrPasswordTooWeak
	}
	return nil
}

// ValidateRegistration là entry point chính — nhận toàn bộ input
// và trả về error nếu bất kỳ rule nào vi phạm
func (p *RegisterPolicy) ValidateRegistration(
	email valueobject.Email,
	password string,
) error {
	if email.IsZero() {
		return err.New("IDENTITY_EMAIL_REQUIRED", "email is required")
	}
	return p.ValidatePassword(password)
}
