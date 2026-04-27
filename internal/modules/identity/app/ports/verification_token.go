package ports

import "context"

type VerificationTokenService interface {
	IssueEmailVerificationToken(ctx context.Context, userID int64) (string, error)
	ParseEmailVerificationToken(ctx context.Context, token string) (int64, error)
}
