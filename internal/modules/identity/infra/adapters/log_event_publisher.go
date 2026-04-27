package adapters

import (
	"context"
	"fmt"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	"go.uber.org/zap"
)

// LogEventPublisher implements ports.EventPublisher.
// Phase 1: chỉ log event ra, không gửi qua Kafka/SMTP thật.
// Khi cần tích hợp thật, thay thế implementation này bằng KafkaEventPublisher hoặc SMTPPublisher.
type LogEventPublisher struct {
	log *zap.Logger
}

func NewLogEventPublisher(log *zap.Logger) ports.EventPublisher {
	return &LogEventPublisher{log: log}
}

func (p *LogEventPublisher) PublishUserRegistered(_ context.Context, payload ports.UserRegisteredPayload) error {
	p.log.Info("event: UserRegistered",
		zap.Int64("user_id", payload.UserID),
		zap.String("email", payload.Email),
	)
	// Phase 1: log only — không gửi email thật.
	// TODO: thay bằng KafkaPublisher hoặc gọi trực tiếp NotificationService khi sẵn sàng.
	if payload.Token == "" {
		return fmt.Errorf("UserRegistered event: verification token is empty")
	}
	return nil
}
