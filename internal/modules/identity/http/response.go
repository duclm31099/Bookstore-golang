package http

import "time"

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type RegisterResponse struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type MeResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	UserType string `json:"user_type"`
	Status   string `json:"status"`
}

type AddressResponse struct {
	ID           int64  `json:"id"`
	Line1        string `json:"line1"`
	Line2        string `json:"line2"`
	ProvinceCode string `json:"province_code"`
	DistrictCode string `json:"district_code"`
	WardCode     string `json:"ward_code"`
	PostalCode   string `json:"postal_code"`
	CountryCode  string `json:"country_code"`
	IsDefault    bool   `json:"is_default"`
}

type SessionResponse struct {
	ID        int64     `json:"id"`
	Device    string    `json:"device"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DeviceResponse struct {
	ID          int64     `json:"id"`
	Fingerprint string    `json:"fingerprint"`
	Label       string    `json:"label"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}
