package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims là dữ liệu được encode vào Access JWT.
// Đây là platform type — không phụ thuộc vào bất kỳ module domain nào.
type JWTClaims struct {
	UserID int64
	Email  string
	Role   string
	Type   string
}

// jwtPayload là internal struct cho jwt.Claims interface.
type jwtPayload struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// JWTTokenManager generate access token (HS256 JWT) và refresh token (opaque random hex).
// Refresh token không mang claims để tránh thông tin lỗi thời — trạng thái lưu trong DB.
type JWTTokenManager struct {
	cfg config.JWTConfig
}

func NewJWTTokenManager(cfg config.JWTConfig) *JWTTokenManager {
	return &JWTTokenManager{cfg: cfg}
}

// GenerateAccessToken tạo HS256 JWT với claims đầy đủ và TTL từ config.
func (m *JWTTokenManager) GenerateAccessToken(_ context.Context, claims JWTClaims) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.cfg.AccessTokenTTL) // 2 hours
	payload := jwtPayload{
		UserID: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
		Type:   string(entity.AccessTokenTypeAccess),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, payload).SignedString([]byte(m.cfg.Secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return token, expiresAt, nil
}

// GenerateRefreshToken tạo opaque random hex 32 bytes.
// Caller tự hash SHA-256 trước khi lưu vào DB.
func (m *JWTTokenManager) GenerateRefreshToken(_ context.Context, _ int64) (string, error) {
	raw, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return raw, nil
}
