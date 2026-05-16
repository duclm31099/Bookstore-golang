package identity

import "fmt"

type DomainError struct {
	Code    string
	Message string
}

func New(code, message string) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
	}
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("[%s]: %s", e.Code, e.Message)
}

// ------------------ User errors -------------
var (
	ErrUserNotFound      = New("IDENTITY_USER_NOT_FOUND", "user not found")
	ErrEmailAlreadyExist = New("IDENTITY_EMAIL_DUPLICATE", "email already exists")
	ErrAccountSuspended  = New("IDENTITY_ACCOUNT_SUSPENDED", "account is suspended")
	ErrAccountBanned     = New("IDENTITY_ACCOUNT_BANNED", "account is banned")
	ErrEmailNotVerified  = New("IDENTITY_EMAIL_NOT_VERIFIED", "email verification required")
)

// ------------------ Credentials errors -----------------------
var (
	ErrCredentialNotFound  = New("IDENTITY_CREDENTIAL_NOT_FOUND", "credential not found")
	ErrInvalidCredentials  = New("IDENTITY_INVALID_CREDENTIALS", "invalid email or password")
	ErrPasswordTooWeak     = New("IDENTITY_PASSWORD_TOO_WEAK", "password does not meet requirements")
	ErrResetTokenExpired   = New("IDENTITY_RESET_TOKEN_EXPIRED", "password reset token is invalid or has expired")
)

// ------------------ Session errors -----------------------
var (
	ErrSessionNotFound = New("IDENTITY_SESSION_NOT_FOUND", "session not found")
	ErrSessionExpired  = New("IDENTITY_SESSION_EXPIRED", "session has expired")
	ErrSessionInvalid  = New("IDENTITY_SESSION_INVALID", "session is invalid")
	ErrSessionRevoked  = New("IDENTITY_SESSION_REVOKED", "session has been revoked")
)

// ------------------ Device errors -----------------------
var (
	ErrDeviceNotFound     = New("IDENTITY_DEVICE_NOT_FOUND", "device not found")
	ErrDeviceRevoked      = New("IDENTITY_DEVICE_REVOKED", "device has been revoked")
	ErrDeviceLimitReached = New("IDENTITY_DEVICE_LIMIT", "maximum device limit reached")
	ErrDeviceNotOwned     = New("IDENTITY_DEVICE_NOT_OWNED", "device is not owned by user")
)

// ------------------ Address errors -----------------------
var (
	ErrAddressNotFound = New("IDENTITY_ADDRESS_NOT_FOUND", "address not found")
	ErrAddressNotOwned = New("IDENTITY_ADDRESS_NOT_OWNED", "address does not belong to this user")
)

// ------------------ Role errors -----------------------
var (
	ErrRoleNotFound = New("IDENTITY_ROLE_NOT_FOUND", "role not found")
)
