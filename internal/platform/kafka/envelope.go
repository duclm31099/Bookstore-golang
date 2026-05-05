package kafka

import (
	"encoding/json"
	"time"
)

// Envelope là chuẩn bao bì cho mọi Kafka message trong hệ thống.
type Envelope struct {
	// Identity & routing
	EventID       string `json:"event_id"`
	EventType     string `json:"event_type"`
	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`

	// Timing
	OccurredAt time.Time `json:"occurred_at"`
	ProducedAt time.Time `json:"produced_at"`

	// Observability
	TraceID       string `json:"trace_id,omitempty"` // Dùng omitempty để tiết kiệm băng thông nếu rỗng
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`

	// Actor
	ActorType string `json:"actor_type"`
	ActorID   int64  `json:"actor_id"`

	// Schema evolution
	SchemaVersion int `json:"schema_version"`

	// Idempotency
	IdempotencyKey string `json:"idempotency_key"`

	// Business payload
	// MASTER FIX: Thay vì []byte, dùng json.RawMessage để Go giữ nguyên format JSON nguyên bản,
	// KHÔNG bị mã hóa sang Base64.
	Payload json.RawMessage `json:"payload"`
}

// NewEnvelope tạo envelope chuẩn mực.
func NewEnvelope(
	eventType string,
	aggregateType string,
	aggregateID string,
	occurredAt time.Time,
	payload []byte, // Nhận []byte từ service (đã json.Marshal)
	opts ...EnvelopeOption,
) Envelope {
	// MASTER FIX: Sinh ID 1 lần duy nhất để dùng chung cho Idempotency default
	eventID := newUUID()

	e := Envelope{
		EventID:        eventID,
		EventType:      eventType,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		OccurredAt:     occurredAt.UTC(), // MASTER FIX: Ép buộc chuẩn hóa về UTC
		ProducedAt:     time.Now().UTC(),
		SchemaVersion:  1,
		IdempotencyKey: eventID, // Chuẩn logic: Default chính là EventID

		// Ép kiểu []byte sang json.RawMessage cực kỳ nhanh, không tốn copy memory
		Payload: json.RawMessage(payload),
	}

	for _, opt := range opts {
		opt(&e)
	}

	return e
}

type EnvelopeOption func(*Envelope)

func WithTraceID(traceID string) EnvelopeOption {
	return func(e *Envelope) { e.TraceID = traceID }
}

func WithCorrelationID(id string) EnvelopeOption {
	return func(e *Envelope) { e.CorrelationID = id }
}

func WithCausationID(id string) EnvelopeOption {
	return func(e *Envelope) { e.CausationID = id }
}

func WithActor(actorType string, actorID int64) EnvelopeOption {
	return func(e *Envelope) {
		e.ActorType = actorType
		e.ActorID = actorID
	}
}

func WithIdKey(key string) EnvelopeOption {
	return func(e *Envelope) { e.IdempotencyKey = key }
}

func WithSchemaVersion(v int) EnvelopeOption {
	return func(e *Envelope) { e.SchemaVersion = v }
}

func newUUID() string {
	// Create random string format like uuid
	return time.Now().String()
}

/*
1. Phân tích Chức năng và Mục đích (Tư duy Master)
	Ý tưởng đằng sau Envelope giống hệt việc bạn gửi một bưu kiện qua bưu điện:
	Bưu điện (Kafka Broker): Không cần (và không nên) mở bưu kiện ra xem bên trong có gì. Nó chỉ cần nhìn vào vỏ hộp (Envelope) để biết gửi đi đâu, ai gửi, gửi lúc nào.
	Món hàng (Payload): Chính là business event của bạn (ví dụ: giỏ hàng, thông tin thanh toán).

2. 3 Quyền năng tối thượng của file này:
	Observability Trio (Bộ ba quan sát): TraceID (Theo dõi xuyên suốt 1 request từ API xuống DB ra Kafka), CorrelationID (Nhóm các request thuộc cùng 1 luồng nghiệp vụ lớn), CausationID (Xác định event A sinh ra event B).
	Thiếu 3 trường này, khi hệ thống có bug ở microservice số 4, bạn sẽ hoàn toàn "mù" và không thể debug được.
	Schema Evolution: Đánh version (SchemaVersion) cho payload. Tương lai payload đổi cấu trúc, Consumer đọc version thấy khác sẽ dùng một bộ parse JSON khác, không bị văng lỗi.
	Functional Options Pattern: Cách bạn viết WithTraceID, WithActor là một chuẩn mực trong Go (giống cách gRPC khởi tạo). Nó giúp hàm NewEnvelope không bị phình to với 20 tham số rườm rà.

3. Tại sao json.RawMessage lại là phép màu?
	Trong thư viện chuẩn của Go, json.RawMessage bản chất vẫn là []byte.
	Tuy nhiên, nó có một "đặc quyền" riêng: Khi bộ json.Marshal gặp kiểu này, nó tự dặn lòng: "Aha, thằng này là JSON thuần rồi, đừng có đụng chạm gì cả, cứ bê nguyên xi mảng byte này nhét thẳng vào chuỗi JSON output đi".
	Nhờ đó, bạn né được lỗi Base64 mà hiệu năng lại cực nhanh!
*/
