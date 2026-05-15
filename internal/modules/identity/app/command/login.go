package command

type LoginCommand struct {
	Email             string
	Password          string
	DeviceFingerprint string
	DeviceLabel       string
	IPAddress         string
	UserAgent         string
}

type ChangePasswordCommand struct {
	UserID           int64
	CurrentPassword  string
	NewPassword      string
	CurrentSessionID int64
}
