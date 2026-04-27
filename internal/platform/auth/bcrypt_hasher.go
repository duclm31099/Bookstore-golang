package auth

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher là platform-level password hasher dùng thuật toán bcrypt.
// Cost nên lấy từ config (mặc định 12 là production-safe).
type BcryptHasher struct {
	cost int
}

func NewBcryptHasher(cost int) *BcryptHasher {
	return &BcryptHasher{cost: cost}
}

// Hash trả về bcrypt hash của raw password.
func (h *BcryptHasher) Hash(_ context.Context, raw string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(raw), h.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify so sánh raw password với bcrypt hash.
func (h *BcryptHasher) Verify(_ context.Context, raw, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
}
