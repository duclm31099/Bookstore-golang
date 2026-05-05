package outbox

import "time"

type State string

const (
	StatePending   State = "pending"
	StatePublished State = "published"
	StateFailed    State = "failed"
)

type OutboxEvent struct {
	ID            int64
	Topic         string
	EventKey      string
	AggregateType string
	AggregateID   *string // VARCHAR(255) trong migration, hỗ trợ cả int và UUID
	EventType     string
	Payload       []byte
	Metadata      []byte // JSONB nullable cho trace/correlation headers
	State         State
	CreatedAt     time.Time
	PublishedAt   *time.Time
	LastError     *string
}

type RecordParams struct {
	Topic          string
	EventKey       string
	AggregateType  string
	AggregateID    *string
	EventType      string
	OccurredAt     time.Time
	TraceID        string // TraceID (Theo dõi xuyên suốt 1 request từ API xuống DB ra Kafka)
	CorrelationID  string // CorrelationID (Nhóm các request thuộc cùng 1 luồng nghiệp vụ lớn)
	CausationID    string // CausationID (Xác định event A sinh ra event B).
	ActorType      string
	ActorID        *int64
	SchemaVersion  int
	IdempotencyKey string
	Payload        map[string]any
}
