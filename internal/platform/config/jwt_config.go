package config

import "time"

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	BcryptCost      int
}

func loadJWTConfig() JWTConfig {
	return JWTConfig{
		Secret:          mustGetEnv("JWT_SECRET"),
		AccessTokenTTL:  time.Duration(getEnvAsInt("JWT_ACCESS_TOKEN_TTL_MINUTES", 120)) * time.Minute,
		RefreshTokenTTL: time.Duration(getEnvAsInt("JWT_REFRESH_TOKEN_TTL_DAYS", 30)) * time.Hour,
		Issuer:          mustGetEnv("JWT_ISSUER"),
		BcryptCost:      getEnvAsInt("BCRYPT_COST", 12),
	}
}
