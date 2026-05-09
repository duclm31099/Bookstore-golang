package auth

import (
	"fmt"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID    int64  `json:"user_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
	jwt.RegisteredClaims
}

const (
	AccessType  = "access"
	RefreshType = "refresh"
)

type Auth struct {
	jwtCfg config.JWTConfig
}

func NewAuthManager(jwtCfg config.JWTConfig) *Auth {
	return &Auth{jwtCfg: jwtCfg}
}

// Generate Access Token
func (a *Auth) GenerateAccessToken(userID int64, email, role, sessionID, deviceID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(a.jwtCfg.AccessTokenTTL)
	raw := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		Type:      AccessType,
		SessionID: sessionID,
		DeviceID:  deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, raw)
	tokenString, err := token.SignedString([]byte(a.jwtCfg.Secret))
	return tokenString, expiresAt, err
}

// SỬA LẠI: Thêm SessionID, DeviceID và lấy TTL từ config
func (a *Auth) GenerateRefreshToken(userID int64, email, role, deviceID string) (string, error) {
	// Giả sử bạn thêm RefreshTokenTTL vào config. Nếu chưa có, nhớ thêm nhé!
	expiresAt := time.Now().Add(a.jwtCfg.RefreshTokenTTL)
	raw := Claims{
		UserID:   userID,
		Email:    email,
		Role:     role,
		Type:     RefreshType,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, raw)
	tk, err := token.SignedString([]byte(a.jwtCfg.Secret))
	return tk, err
}

// Hàm core dùng chung để parse và validate chữ ký JWT
func (a *Auth) parseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(a.jwtCfg.Secret), nil
	})

	if err != nil {
		return nil, err // Bao gồm cả lỗi token hết hạn (TokenExpired)
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// Validate access token gọn gàng hơn
func (a *Auth) ValidateAccessToken(token string) (*Claims, error) {
	claims, err := a.parseToken(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != AccessType {
		return nil, fmt.Errorf("invalid token type: expected access")
	}

	return claims, nil
}

// Validate refresh token gọn gàng hơn
func (a *Auth) ValidateRefreshToken(token string) (*Claims, error) {
	claims, err := a.parseToken(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != RefreshType {
		return nil, fmt.Errorf("invalid token type: expected refresh")
	}

	return claims, nil
}

func (a *Auth) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), a.jwtCfg.BcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (a *Auth) VerifyPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
