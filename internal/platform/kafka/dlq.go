package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// DLQPublisher gửi failed messages vào Dead Letter Queue topic.
//
// Tư duy: DLQ không phải nơi "vứt rác" — nó là safety net có cấu trúc.
// Ops team có thể replay từ DLQ sau khi fix bug,
// hoặc alert dựa trên DLQ growth rate.
type DLQPublisher struct {
	producer *Producer
	topic    string
	log      *zap.Logger
}

func NewDLQPublisher(producer *Producer, topic string, log *zap.Logger) *DLQPublisher {
	return &DLQPublisher{
		producer: producer,
		topic:    topic,
		log:      log,
	}
}

// DLQEnvelope bọc original message kèm failure context.
// Giữ nguyên original payload để có thể replay.
type DLQEnvelope struct {
	OriginalTopic   string          `json:"original_topic"`
	OriginalKey     string          `json:"original_key"`
	OriginalPayload json.RawMessage `json:"original_payload"`
	OriginalHeaders []HeaderPair    `json:"original_headers"`
	FailureReason   string          `json:"failure_reason"`
	FailureError    string          `json:"failure_error"`
	FailedAt        time.Time       `json:"failed_at"`
	ConsumerGroup   string          `json:"consumer_group"`
	Attempt         int             `json:"attempt"`
}

type HeaderPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Publish gửi một failed message lên DLQ topic.
// Dùng original key để DLQ có thể partition đúng khi replay.
func (d *DLQPublisher) Publish(ctx context.Context, original kafka.Message, reason string, err error) error {
	headers := make([]HeaderPair, len(original.Headers))
	for i, h := range original.Headers {
		headers[i] = HeaderPair{Key: h.Key, Value: string(h.Value)}
	}

	dlqEnv := DLQEnvelope{
		OriginalTopic:   original.Topic,
		OriginalKey:     string(original.Key),
		OriginalPayload: json.RawMessage(original.Value),
		OriginalHeaders: headers,
		FailureReason:   reason,
		FailureError:    errorString(err),
		FailedAt:        time.Now().UTC(),
		Attempt:         1,
	}

	body, marshalErr := json.Marshal(dlqEnv)
	if marshalErr != nil {
		return fmt.Errorf("dlq: marshal failed: %w", marshalErr)
	}

	dlqMsg := kafka.Message{
		Topic: d.topic,
		Key:   original.Key, // Giữ nguyên key để ordering trong DLQ
		Value: body,
		Headers: []kafka.Header{
			{Key: "x-original-topic", Value: []byte(original.Topic)},
			{Key: "x-failure-reason", Value: []byte(reason)},
			{Key: "x-failed-at", Value: []byte(dlqEnv.FailedAt.Format(time.RFC3339))},
		},
	}

	if writeErr := d.producer.writer.WriteMessages(ctx, dlqMsg); writeErr != nil {
		d.log.Error("dlq: failed to publish to DLQ",
			zap.String("original_topic", original.Topic),
			zap.String("reason", reason),
			zap.Error(writeErr),
		)
		return fmt.Errorf("dlq: write failed: %w", writeErr)
	}

	d.log.Warn("dlq: message sent to DLQ",
		zap.String("original_topic", original.Topic),
		zap.String("reason", reason),
		zap.String("original_key", string(original.Key)),
	)

	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
