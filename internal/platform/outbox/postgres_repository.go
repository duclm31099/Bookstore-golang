package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/db"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) executor(ctx context.Context) db.Executor {
	return tx.GetExecutor(ctx, r.pool)
}

func (r *PostgresRepository) Insert(ctx context.Context, e OutboxEvent) error {
	const q = `
		INSERT INTO outbox_events (topic, event_key, aggregate_type, aggregate_id, event_type, payload, metadata, state, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.executor(ctx).Exec(ctx, q,
		e.Topic, e.EventKey, e.AggregateType, e.AggregateID,
		e.EventType, e.Payload, e.Metadata, e.State, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("outbox repo: insert: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ClaimPending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	const q = `
		UPDATE outbox_events
		SET state = 'publishing'
		WHERE id IN (
			SELECT id FROM outbox_events
			WHERE state = 'pending'
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, event_key, aggregate_type, aggregate_id, event_type, payload, metadata, state, created_at, published_at, last_error
	`

	rows, err := r.executor(ctx).Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox repo: claim pending: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(
			&e.ID, &e.Topic, &e.EventKey, &e.AggregateType, &e.AggregateID,
			&e.EventType, &e.Payload, &e.Metadata, &e.State, &e.CreatedAt, &e.PublishedAt, &e.LastError,
		); err != nil {
			return nil, fmt.Errorf("outbox repo: scan: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *PostgresRepository) MarkPublished(ctx context.Context, id int64) error {
	const q = `UPDATE outbox_events SET state = 'published', published_at = $2 WHERE id = $1`
	tag, err := r.executor(ctx).Exec(ctx, q, id, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("outbox repo: mark published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	const q = `UPDATE outbox_events SET state = 'failed', last_error = $2 WHERE id = $1`
	tag, err := r.executor(ctx).Exec(ctx, q, id, errMsg)
	if err != nil {
		return fmt.Errorf("outbox repo: mark failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}
