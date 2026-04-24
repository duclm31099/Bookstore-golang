package config

import "time"

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func loadRedisConfig() RedisConfig {
	return RedisConfig{
		Addr:         mustGetEnv("REDIS_HOST") + ":" + mustGetEnv("REDIS_PORT"),
		Password:     mustGetEnv("REDIS_PASSWORD"),
		DB:           getEnvAsInt("REDIS_DB", 0),
		MaxRetries:   getEnvAsInt("REDIS_MAX_RETRIES", 3),
		PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 10),
		MinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 5),
		ReadTimeout:  time.Duration(getEnvAsInt("REDIS_READ_TIMEOUT_SECONDS", 5)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("REDIS_WRITE_TIMEOUT_SECONDS", 5)) * time.Second,
		IdleTimeout:  time.Duration(getEnvAsInt("REDIS_DIAL_TIMEOUT_SECONDS", 5)) * time.Second,
	}
}
