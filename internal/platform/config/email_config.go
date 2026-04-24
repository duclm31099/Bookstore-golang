package config

type EmailConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	UseTLS    bool
}

func loadEmailConfig() EmailConfig {
	return EmailConfig{
		Host:      mustGetEnv("SMTP_HOST"),
		Port:      getEnvAsInt("SMTP_PORT", 1025),
		Username:  mustGetEnv("SMTP_USER"),
		Password:  mustGetEnv("SMTP_PASSWORD"),
		FromEmail: mustGetEnv("SMTP_FROM_EMAIL"),
		FromName:  mustGetEnv("SMTP_FROM_NAME"),
		UseTLS:    getEnvAsBool("SMTP_TLS", false),
	}
}
