package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/kafka"
	"go.uber.org/zap"
)

// KafkaPublisher adapts kafka.Producer to the outbox.Publisher interface.
type KafkaPublisher struct {
	producer *kafka.Producer
	log      *zap.Logger
}

func NewKafkaPublisher(producer *kafka.Producer, log *zap.Logger) *KafkaPublisher {
	return &KafkaPublisher{producer: producer, log: log}
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic string, key string, payload []byte) error {
	// payload ở đây chính là JSON của Envelope đã marshal sẵn từ Recorder.
	// Ta parse lại thành outbox.Envelope để build kafka.Envelope chuẩn.
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("outbox publisher: unmarshal envelope: %w", err)
	}

	payloadBytes, err := json.Marshal(env.Payload)
	if err != nil {
		return fmt.Errorf("outbox publisher: marshal payload: %w", err)
	}

	// Resolve aggregate_id: outbox dùng *string, kafka dùng string
	var aggID string
	if env.AggregateID != nil {
		aggID = *env.AggregateID
	}

	kafkaEnv := kafka.NewEnvelope(
		env.EventType,
		env.AggregateType,
		aggID,
		env.OccurredAt,
		payloadBytes,
		kafka.WithSchemaVersion(env.SchemaVersion),
		kafka.WithIdKey(env.IdempotencyKey),
	)
	// Override produced_at to now (thời điểm thực sự publish)
	kafkaEnv.ProducedAt = time.Now().UTC()
	// Set EventID ổn định từ outbox envelope (đã sinh 1 lần duy nhất khi Record)
	kafkaEnv.EventID = env.EventID
	kafkaEnv.IdempotencyKey = env.IdempotencyKey

	// Observability
	if env.TraceID != "" {
		kafkaEnv.TraceID = env.TraceID
	}
	if env.CorrelationID != "" {
		kafkaEnv.CorrelationID = env.CorrelationID
	}
	if env.CausationID != "" {
		kafkaEnv.CausationID = env.CausationID
	}
	if env.ActorType != "" {
		kafkaEnv.ActorType = env.ActorType
		if env.ActorID != nil {
			kafkaEnv.ActorID = *env.ActorID
		}
	}

	return p.producer.PublishEnvelope(ctx, topic, kafkaEnv)
}
