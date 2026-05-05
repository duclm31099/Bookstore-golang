package outbox

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// OutboxDispatcher đọc outbox_events pending, publish lên Kafka, rồi đánh dấu published/failed.
// Được gọi bởi Worker hoặc Scheduler theo polling interval.
type OutboxDispatcher struct {
	repo      Repository
	publisher Publisher
	log       *zap.Logger
}

func NewDispatcher(repo Repository, publisher Publisher, log *zap.Logger) *OutboxDispatcher {
	return &OutboxDispatcher{
		repo:      repo,
		publisher: publisher,
		log:       log,
	}
}

// DispatchOnce claim tối đa `limit` events pending rồi publish từng cái.
// Trả về số event đã publish thành công.
func (d *OutboxDispatcher) DispatchOnce(ctx context.Context, limit int) (int, error) {
	events, err := d.repo.ClaimPending(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("outbox dispatcher: claim: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	published := 0
	for _, e := range events {
		if err := d.publisher.Publish(ctx, e.Topic, e.EventKey, e.Payload); err != nil {
			d.log.Error("outbox dispatcher: publish failed",
				zap.Int64("outbox_id", e.ID),
				zap.String("topic", e.Topic),
				zap.String("event_type", e.EventType),
				zap.Error(err),
			)
			if markErr := d.repo.MarkFailed(ctx, e.ID, err.Error()); markErr != nil {
				d.log.Error("outbox dispatcher: mark failed also failed",
					zap.Int64("outbox_id", e.ID),
					zap.Error(markErr),
				)
			}
			continue
		}

		if err := d.repo.MarkPublished(ctx, e.ID); err != nil {
			d.log.Error("outbox dispatcher: mark published failed",
				zap.Int64("outbox_id", e.ID),
				zap.Error(err),
			)
			continue
		}

		published++
	}

	d.log.Debug("outbox dispatcher: batch complete",
		zap.Int("claimed", len(events)),
		zap.Int("published", published),
	)
	return published, nil
}
