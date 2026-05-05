package outbox

import "time"

// Envelope là cấu trúc dữ liệu chuẩn cho message Kafka,
// được marshal thành JSON và lưu vào outbox_events.payload.
type Envelope struct {
	EventID        string         `json:"event_id"`
	EventType      string         `json:"event_type"`
	AggregateType  string         `json:"aggregate_type"`
	AggregateID    *string        `json:"aggregate_id,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
	ProducedAt     time.Time      `json:"produced_at"`
	TraceID        string         `json:"trace_id,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
	CausationID    string         `json:"causation_id,omitempty"`
	ActorType      string         `json:"actor_type,omitempty"`
	ActorID        *int64         `json:"actor_id,omitempty"`
	SchemaVersion  int            `json:"schema_version"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Payload        map[string]any `json:"payload"`
}
