package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App         AppConfig
	DB          DBConfig
	Redis       RedisConfig
	Kafka       KafkaConfig
	Minio       MinioConfig
	JWT         JWTConfig
	Email       EmailConfig
	VNPay       VNPayConfig
	AsynqConfig AsynqConfig
	RateLimit   RateLimitConfig
	CORS        CORSConfig
	Logger      LoggerConfig
}

func MustLoad() *Config {
	_ = godotenv.Load()
	cfg := &Config{
		DB:          loadDBConfig(),
		App:         loadAppConfig(),
		Redis:       loadRedisConfig(),
		Kafka:       loadKafkaConfig(),
		Minio:       loadMinioConfig(),
		JWT:         loadJWTConfig(),
		Email:       loadEmailConfig(),
		VNPay:       loadVNPayConfig(),
		AsynqConfig: loadAsynqConfig(),
		RateLimit:   loadRateLimitConfig(),
		CORS:        loadCORSConfig(),
		Logger:      loadLoggerConfig(),
	}

	cfg.validate()
	return cfg
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}

func getEnvAsInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvAsBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func (c *Config) validate() {
	if c.JWT.Secret == "" {
		panic("missing JWT secret")
	}
	if c.DB.Host == "" || c.DB.Name == "" || c.DB.User == "" {
		panic("missing database config")
	}
}
