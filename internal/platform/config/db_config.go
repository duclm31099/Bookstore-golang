package config

import (
	"fmt"
	"time"
)

type DBConfig struct {
	Host              string
	Port              string
	Name              string
	User              string
	Password          string
	SSLMode           string
	MaxOpenConns      int32
	MinIdleConns      int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthcheckPeriod time.Duration
	MigrationDir      string
}

func loadDBConfig() DBConfig {
	return DBConfig{
		Host:              mustGetEnv("DB_HOST"),
		Port:              mustGetEnv("DB_PORT"),
		Name:              mustGetEnv("DB_NAME"),
		User:              mustGetEnv("DB_USER"),
		Password:          mustGetEnv("DB_PASSWORD"),
		SSLMode:           mustGetEnv("DB_SSL_MODE"),
		MaxOpenConns:      int32(getEnvAsInt("DB_MAX_OPEN_CONNS", 25)),
		MinIdleConns:      int32(getEnvAsInt("DB_MIN_IDLE_CONNS", 5)),
		MaxConnLifetime:   time.Duration(getEnvAsInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
		MaxConnIdleTime:   time.Duration(getEnvAsInt("DB_CONN_MAX_IDLE_TIME_MINUTES", 10)) * time.Minute,
		HealthcheckPeriod: time.Duration(getEnvAsInt("DB_HEALTHCHECK_PERIOD_SECONDS", 30)) * time.Second,
		MigrationDir:      mustGetEnv("DB_MIGRATION_DIR"),
	}
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}
