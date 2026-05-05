package outbox

import "context"

type Repository interface {
	Insert(ctx context.Context, e OutboxEvent) error
	ClaimPending(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}

type Publisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte) error
}

type Recorder interface {
	Record(ctx context.Context, params RecordParams) error
}

type Dispatcher interface {
	DispatchOnce(ctx context.Context, limit int) (int, error)
}
