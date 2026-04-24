package config

import "time"

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

func loadCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:8080",
		},
		AllowedMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
		},
		AllowedHeaders: []string{
			"Content-Type",
			"Authorization",
			"X-Request-ID",
			"X-Idempotency-Key",
		},
		AllowCredentials: true,
		MaxAge:           int(time.Hour * 24),
	}
}
