package config

type AsynqConfig struct {
	RedisAddr        string
	RedisDB          int
	RedisPass        string
	Concurrency      int
	Queues           map[string]int
	AsynqMonitorPort int
}

func loadAsynqConfig() AsynqConfig {
	return AsynqConfig{
		RedisAddr:   mustGetEnv("REDIS_HOST") + ":" + mustGetEnv("REDIS_PORT"),
		RedisDB:     getEnvAsInt("REDIS_DB", 1),
		RedisPass:   mustGetEnv("REDIS_PASSWORD"),
		Concurrency: getEnvAsInt("ASYNQ_CONCURRENCY", 10),
		Queues: map[string]int{
			"default":  getEnvAsInt("ASYNQ_QUEUES_DEFAULT", 3),
			"critical": getEnvAsInt("ASYNQ_QUEUES_CRITICAL", 6),
		},
		AsynqMonitorPort: getEnvAsInt("ASYNQ_MONITOR_PORT", 8081),
	}
}
