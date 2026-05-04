package redis

import "time"

type Config struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	KeyPrefix    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	MinIdleConns int
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = "127.0.0.1:6379"
	}
	if c.KeyPrefix == "" {
		c.KeyPrefix = "app"
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = 3 * time.Second
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 2 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 2 * time.Second
	}
	if c.PoolSize <= 0 {
		c.PoolSize = 20
	}
	return c
}
