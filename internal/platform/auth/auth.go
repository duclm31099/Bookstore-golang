package auth

import (
	"fmt"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ------------------ JWT USE CASE -----------------------
// 1. Xác thực -> Verify/Generate access/refresh token
// 2. Ủy quyền -> Check permission

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

type Auth struct {
	jwtCfg config.JWTConfig
}

func NewAuthManager(jwtCfg config.JWTConfig) *Auth {
	return &Auth{jwtCfg: jwtCfg}
}

// Generate Token - a new JWT token for the given user ID.
func (a *Auth) GenerateAccessToken(userID int64, email, role string) (string, error) {
	expiresAt := time.Now().Add(a.jwtCfg.AccessTokenTTL)
	raw := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, raw)
	return token.SignedString([]byte(a.jwtCfg.Secret))
}

// Generate refresh token
func (a *Auth) GenerateRefreshToken(userID int64, email, role string) (string, error) {
	raw := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, raw)
	return token.SignedString([]byte(a.jwtCfg.Secret))
}

// Validate access token
func (a *Auth) ValidateAccessToken(token string) (*Claims, error) {
	claims := &Claims{}

	parsedToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(a.jwtCfg.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Type != "access" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil

}

// Validate refresh token
func (a *Auth) ValidateRefreshToken(token string) (*Claims, error) {
	claims := &Claims{}

	parsedToken, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(a.jwtCfg.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.Type != "refresh" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

func (a *Auth) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), a.jwtCfg.BcryptCost)
	return string(bytes), err
}

func (a *Auth) VerifyPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
