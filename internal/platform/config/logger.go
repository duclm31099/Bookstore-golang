package config

type LoggerConfig struct {
	Level  string
	Format string
	Output string
}

func loadLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:  mustGetEnv("LOG_LEVEL"),
		Format: mustGetEnv("LOG_FORMAT"),
		Output: mustGetEnv("LOG_OUTPUT"),
	}
}
