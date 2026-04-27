package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateSecureToken tạo random hex string 32 bytes (64 ký tự).
// Dùng chung cho refresh token và email verification token.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	return hex.EncodeToString(b), nil
}
