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
	DeviceFingerprint string
	DeviceLabel       string
	IPAddress         *string
	UserAgent         string
}

type LoginOutput struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type RefreshTokenInput struct {
	RefreshToken string
}

type RefreshTokenOutput struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type LogoutInput struct {
	SessionID int64
}

type VerifyEmailInput struct {
	Token string
}
