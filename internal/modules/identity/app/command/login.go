package command

type LoginCommand struct {
	Email             string
	Password          string
	DeviceFingerprint *string
	DeviceLabel       *string
	IPAddress         string
	UserAgent         string
}
