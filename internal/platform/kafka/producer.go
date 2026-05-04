package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Producer là sync Kafka producer dùng cho outbox relay.
type Producer struct {
	writer *kafka.Writer
	log    *zap.Logger
}

func NewProducer(cfg Config, log *zap.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.Hash{}, // Partition by key hash (Đảm bảo ordering)
		RequiredAcks: kafka.RequiredAcks(cfg.ProducerRequiredAcks),
		MaxAttempts:  cfg.ProducerMaxRetries,
		WriteTimeout: cfg.ProducerWriteTimeout,

		// MASTER FIX: Bơm cấu hình Batching đã định nghĩa vào Writer
		BatchBytes:   int64(cfg.ProducerBatchSize),
		BatchTimeout: cfg.ProducerLingerMs,

		AllowAutoTopicCreation: false,
		Async:                  false, // MASTER RULE: Luôn false cho Outbox Pattern
	}

	return &Producer{
		writer: writer,
		log:    log,
	}
}

// PublishEnvelope serialize envelope thành JSON và gửi 1 message.
func (p *Producer) PublishEnvelope(ctx context.Context, topic string, e Envelope) error {
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("kafka producer: marshal envelope: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		// MASTER FIX: Ép kiểu e.AggregateType string về chuẩn AggregateType để compiler không báo lỗi
		Key:     []byte(EventKey(AggregateType(e.AggregateType), e.AggregateID)),
		Value:   body,
		Headers: BuildHeaders(e),
		Time:    e.ProducedAt,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		// Lỗi thì log ở mức Error là hoàn toàn chính xác
		p.log.Error("kafka producer: write failed",
			zap.String("topic", topic),
			zap.String("event_type", e.EventType),
			zap.String("event_id", e.EventID),
			zap.Error(err),
		)
		return fmt.Errorf("kafka producer: write message [%s/%s]: %w", topic, e.EventType, err)
	}

	// MASTER FIX: Hạ cấp độ log thành Debug.
	// Trên môi trường Production (thường chạy mức Info), dòng này sẽ bị bỏ qua, cứu sống ổ cứng của bạn.
	p.log.Debug("kafka producer: message published",
		zap.String("topic", topic),
		zap.String("event_id", e.EventID),
	)

	return nil
}

// PublishBatch gửi nguyên một mảng messages trong 1 Network Call.
func (p *Producer) PublishBatch(ctx context.Context, messages []TopicMessage) error {
	if len(messages) == 0 {
		return nil
	}

	kmsgs := make([]kafka.Message, 0, len(messages))

	for _, tm := range messages {
		body, err := json.Marshal(tm.Envelope)
		if err != nil {
			// Nếu marshal 1 phần tử lỗi, lập tức fail toàn bộ batch để đảm bảo tính toàn vẹn Outbox
			return fmt.Errorf("kafka producer: marshal envelope [%s]: %w", tm.Envelope.EventType, err)
		}

		kmsgs = append(kmsgs, kafka.Message{
			Topic:   tm.Topic,
			Key:     []byte(EventKey(AggregateType(tm.Envelope.AggregateType), tm.Envelope.AggregateID)),
			Value:   body,
			Headers: BuildHeaders(tm.Envelope),
			Time:    tm.Envelope.ProducedAt,
		})
	}

	// Viết cả batch lên mạng trong 1 lần duy nhất
	if err := p.writer.WriteMessages(ctx, kmsgs...); err != nil {
		p.log.Error("kafka producer: batch write failed",
			zap.Int("batch_size", len(messages)),
			zap.Error(err),
		)
		return fmt.Errorf("kafka producer: batch write failed: %w", err)
	}

	p.log.Debug("kafka producer: batch published successfully", zap.Int("batch_size", len(messages)))
	return nil
}

// TopicMessage gom topic và envelope lại để dùng trong batch.
type TopicMessage struct {
	Topic    string
	Envelope Envelope
}

func (p *Producer) Close() error {
	p.log.Info("kafka producer: closing gracefully")
	return p.writer.Close()
}
