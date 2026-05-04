package kafka

import (
	"github.com/google/wire"
	"go.uber.org/zap"
)

var ProviderSet = wire.NewSet(
	ProvideProducer,
	ProvideDLQPublisher,
	ProvideConsumerGroup,
)

func ProvideProducer(cfg Config, log *zap.Logger) *Producer {
	return NewProducer(cfg, log)
}

func ProvideDLQPublisher(producer *Producer, cfg Config, log *zap.Logger) *DLQPublisher {
	return NewDLQPublisher(producer, cfg.DLQTopic, log)
}

func ProvideConsumerGroup(cfg Config, log *zap.Logger, dlq *DLQPublisher) *ConsumerGroup {
	return NewConsumerGroup(cfg, log, dlq)
}
