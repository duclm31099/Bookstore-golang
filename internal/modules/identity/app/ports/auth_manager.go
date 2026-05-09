package ports

import (
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
)

type AuthManager interface {
	GenerateAccessToken(userID int64, email, role, sessionID, deviceID string) (string, time.Time, error)
	GenerateRefreshToken(userID int64, email, role, deviceID string) (string, error)
	ValidateAccessToken(token string) (*auth.Claims, error)
	ValidateRefreshToken(token string) (*auth.Claims, error)
	HashPassword(password string) (string, error)
	VerifyPassword(password string, hash string) error
}
