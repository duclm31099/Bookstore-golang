package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// OutboxRecorder ghi event vào bảng outbox_events cùng transaction nghiệp vụ.
// Service layer gọi Record() bên trong tx.WithinTransaction để đảm bảo atomicity.
type OutboxRecorder struct {
	repo Repository
}

func NewRecorder(repo Repository) *OutboxRecorder {
	return &OutboxRecorder{repo: repo}
}

func (s *OutboxRecorder) Record(ctx context.Context, params RecordParams) error {
	envelope := Envelope{
		EventID:        uuid.NewString(),
		EventType:      params.EventType,
		AggregateType:  params.AggregateType,
		AggregateID:    params.AggregateID,
		OccurredAt:     params.OccurredAt.UTC(),
		ProducedAt:     time.Time{}, // Sẽ được set khi Dispatcher thực sự publish
		TraceID:        params.TraceID,
		CorrelationID:  params.CorrelationID,
		CausationID:    params.CausationID,
		ActorType:      params.ActorType,
		ActorID:        params.ActorID,
		SchemaVersion:  params.SchemaVersion,
		IdempotencyKey: params.IdempotencyKey,
		Payload:        params.Payload,
	}

	payloadJSON, err := json.Marshal(envelope)
	log.Println("outbox payload:: ", string(payloadJSON))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMarshal, err)
	}

	// Metadata column: lưu trace context riêng để query/debug dễ mà không cần parse payload
	var metadataJSON []byte
	if params.TraceID != "" || params.CorrelationID != "" || params.CausationID != "" {
		meta := map[string]string{}
		if params.TraceID != "" {
			meta["trace_id"] = params.TraceID
		}
		if params.CorrelationID != "" {
			meta["correlation_id"] = params.CorrelationID
		}
		if params.CausationID != "" {
			meta["causation_id"] = params.CausationID
		}
		metadataJSON, _ = json.Marshal(meta)
	}

	event := OutboxEvent{
		Topic:         params.Topic,
		EventKey:      params.EventKey,
		AggregateType: params.AggregateType,
		AggregateID:   params.AggregateID,
		EventType:     params.EventType,
		Payload:       payloadJSON,
		Metadata:      metadataJSON,
		State:         StatePending,
		CreatedAt:     time.Now().UTC(),
	}
	log.Println("outbox recorded:: ", event)

	return s.repo.Insert(ctx, event)
}
