package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}
type PostgresProcessedEventStore struct {
	db DBTX
}

func NewProcessedEventStore(db DBTX) *PostgresProcessedEventStore {
	return &PostgresProcessedEventStore{db: db}
}

func (s *PostgresProcessedEventStore) TryMarkProcessed(
	ctx context.Context,
	consumerName string,
	eventID string,
	processedAt time.Time,
	metadata map[string]any,
) (bool, error) {

	rawMeta, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}

	const q = `
		INSERT INTO processed_events (
			consumer_name,
			event_id,
			processed_at,
			metadata
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (consumer_name, event_id) DO NOTHING
	`

	tag, err := s.db.Exec(ctx, q, consumerName, eventID, processedAt, rawMeta)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() == 1, nil
}
