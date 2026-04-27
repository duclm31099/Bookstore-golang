package ports

import "context"

type UserRegisteredPayload struct {
	UserID int64
	Email  string
	Token  string
}

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, payload UserRegisteredPayload) error
}
