package ports

import "context"

type UserRegisteredPayload struct {
	UserID int64
	Email  string
	Token  string
}
type ResetPasswordRequestedPayload struct {
	UserID int64
	Email  string
	Token  string
}

type ResetPasswordCompletedPayload struct {
	UserID int64
	Email  string
}

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, payload UserRegisteredPayload) error
	PublishResetPasswordRequested(ctx context.Context, payload ResetPasswordRequestedPayload) error
	PublishResetPasswordCompleted(ctx context.Context, payload ResetPasswordCompletedPayload) error
}
