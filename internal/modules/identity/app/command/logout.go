package command

type LogoutCommand struct {
	DeviceID int64
	UserID   int64
}

type ForgotPasswordCommand struct {
	Email string
}

type ResetPasswordCommand struct {
	Token       string
	NewPassword string
}
