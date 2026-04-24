package config

import (
	"strings"
	"time"
)

type KafkaConfig struct {
	Brokers         []string
	ConsumerGroupID string
	ClientID        string
	MaxRetry        int
	RetryBackoff    time.Duration
	ProducerTimeout time.Duration
	ConsumerTimeout time.Duration
	TopicPrefix     string
}

func loadKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:         strings.Split(mustGetEnv("KAFKA_BROKERS"), ""), // localhost:9092
		ConsumerGroupID: mustGetEnv("KAFKA_CONSUMER_GROUP_ID"),
		ClientID:        mustGetEnv("KAFKA_CLIENT_ID"),
		MaxRetry:        getEnvAsInt("KAFKA_MAX_RETRY", 3),
		RetryBackoff:    time.Duration(getEnvAsInt("KAFKA_RETRY_BACKOFF_MS", 500)) * time.Millisecond,
		ProducerTimeout: time.Duration(getEnvAsInt("KAFKA_PRODUCER_TIMEOUT_MS", 5000)) * time.Millisecond,
		ConsumerTimeout: time.Duration(getEnvAsInt("KAFKA_CONSUMER_TIMEOUT_MS", 10000)) * time.Millisecond,
		TopicPrefix:     mustGetEnv("KAFKA_TOPIC_PREFIX"),
	}
}
