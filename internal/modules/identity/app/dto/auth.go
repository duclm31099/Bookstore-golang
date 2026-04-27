package dto

import "time"

type RegisterInput struct {
	Email    string
	Password string
	FullName string
	Phone    *string
}

type RegisterOutput struct {
	UserID  int64
	Email   string
	Message string
}

type LoginInput struct {
	Email             string
	Password          string
	DeviceFingerprint *string
	DeviceLabel       *string
	IPAddress         string
	UserAgent         string
}

type LoginOutput struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type RefreshTokenInput struct {
	RefreshToken string
}

type RefreshTokenOutput struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type LogoutInput struct {
	SessionID int64
}

type VerifyEmailInput struct {
	Token string
}
