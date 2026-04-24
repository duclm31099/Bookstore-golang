package config

import "time"

type AppConfig struct {
	Env                     string
	Name                    string
	Port                    string
	Version                 string
	Debug                   bool
	GracefulShutdownTimeout time.Duration
}

func loadAppConfig() AppConfig {
	return AppConfig{
		Env:                     mustGetEnv("APP_ENV"),
		Name:                    mustGetEnv("APP_NAME"),
		Port:                    mustGetEnv("APP_PORT"),
		Version:                 mustGetEnv("APP_VERSION"),
		Debug:                   getEnvAsBool("APP_DEBUG", false),
		GracefulShutdownTimeout: time.Duration(getEnvAsInt("APP_GRACEFUL_SHUTDOWN_TIMEOUT_SECONDS", 10)) * time.Second,
	}
}
