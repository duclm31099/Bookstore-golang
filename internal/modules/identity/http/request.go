package http

const (
	RefreshTokenCookieName     = "refresh_token"
	RefreshTokenCookiePath     = "/api/v1/auth/refresh-token"
	RefreshTokenCookieDomain   = ""
	RefreshTokenCookieSecure   = false
	RefreshTokenCookieHttpOnly = true
)

type RegisterRequest struct {
	Email    string  `json:"email" binding:"required,email,max=255"`
	Password string  `json:"password" binding:"required,min=8,max=72"`
	FullName string  `json:"full_name" binding:"required"`
	Phone    *string `json:"phone" binding:"omitempty"`
}

type LoginRequest struct {
	Email             string `json:"email" binding:"required,email"`
	Password          string `json:"password" binding:"required"`
	DeviceFingerprint string `json:"device_fingerprint" binding:"required"`
	DeviceLabel       string `json:"device_label" binding:"required"`
}

type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

type LogoutRequest struct {
	SessionID int64 `json:"session_id" binding:"required"`
}

type AddressRequest struct {
	RecipientName  string `json:"recipient_name" binding:"required,max=255"`
	RecipientPhone string `json:"recipient_phone" binding:"required,max=20"`
	Line1          string `json:"line1" binding:"required"`
	Line2          string `json:"line2"`
	ProvinceCode   string `json:"province_code" binding:"required"`
	DistrictCode   string `json:"district_code" binding:"required"`
	WardCode       string `json:"ward_code" binding:"required"`
	PostalCode     string `json:"postal_code"`
	CountryCode    string `json:"country_code"`
	IsDefault      bool   `json:"is_default"`
}
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}
