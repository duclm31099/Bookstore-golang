package config

type RateLimitConfig struct {
	Enabled          bool
	RequestPerMinute int
	BurstSize        int
}

func loadRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:          getEnvAsBool("RATE_LIMIT_ENABLED", true),
		RequestPerMinute: getEnvAsInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 100),
		BurstSize:        getEnvAsInt("RATE_LIMIT_BURST_SIZE", 20),
	}
}
