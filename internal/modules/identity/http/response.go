package http

import "time"

const (
	// RESPONSE MESSAGES
	RegisterSuccess          = "REGISTER_SUCCESS"
	LoginSuccess             = "LOGIN_SUCCESS"
	RefreshTokenSuccess      = "REFRESH_TOKEN_SUCCESS"
	VerifyEmailSuccess       = "VERIFY_EMAIL_SUCCESS"
	LogoutSuccess            = "LOGOUT_SUCCESS"
	RevokeAllSessionsSuccess = "REVOKE_ALL_SESSIONS_SUCCESS"
	RevokeDeviceSuccess      = "REVOKE_DEVICE_SUCCESS"
	ListSessionsSuccess      = "LIST_SESSIONS_SUCCESS"
	ListDevicesSuccess       = "LIST_DEVICES_SUCCESS"
	ListAddressesSuccess     = "LIST_ADDRESSES_SUCCESS"
	AddAddressSuccess        = "ADD_ADDRESS_SUCCESS"
	UpdateAddressSuccess     = "UPDATE_ADDRESS_SUCCESS"
	DeleteAddressSuccess     = "DELETE_ADDRESS_SUCCESS"

	NotAuthenticated = "NOT_AUTHENTICATED"
	SessionRevoked   = "SESSION_REVOKED"
	ValidationErr    = "VALIDATION_ERROR"

	// DETAIL MESSAGE
	RegisterSuccessMessage          = "Registration successful, please verify your email"
	LoginSuccessMessage             = "Login successful"
	RefreshTokenSuccessMessage      = "Refresh token successful"
	VerifyEmailSuccessMessage       = "Verify email successful"
	LogoutSuccessMessage            = "Logout successful"
	RevokeAllSessionsSuccessMessage = "Revoke all sessions successful"
	RevokeDeviceSuccessMessage      = "Revoke device successful"
	ListSessionsSuccessMessage      = "List sessions successful"
	ListDevicesSuccessMessage       = "List devices successful"
	ListAddressesSuccessMessage     = "List addresses successful"
	AddAddressSuccessMessage        = "Add address successful"
	UpdateAddressSuccessMessage     = "Update address successful"
	DeleteAddressSuccessMessage     = "Delete address successful"

	MissingRefreshTokenMessage = "missing refresh token"
	MissingAuthContextMessage  = "missing auth context"
)

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
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type RefreshTokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
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
