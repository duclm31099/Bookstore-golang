package ports

import "context"

type PasswordHasher interface {
	Hash(ctx context.Context, raw string) (string, error)
	Verify(ctx context.Context, raw string, hash string) error
}
