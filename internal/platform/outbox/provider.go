package outbox

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/kafka"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func ProvideRepository(pool *pgxpool.Pool) *PostgresRepository {
	return NewPostgresRepository(pool)
}

func ProvideRecorder(repo *PostgresRepository) *OutboxRecorder {
	return NewRecorder(repo)
}

func ProvidePublisher(producer *kafka.Producer, log *zap.Logger) *KafkaPublisher {
	return NewKafkaPublisher(producer, log)
}

func ProvideDispatcher(repo *PostgresRepository, pub *KafkaPublisher, log *zap.Logger) *OutboxDispatcher {
	return NewDispatcher(repo, pub, log)
}

var ProviderSet = wire.NewSet(
	ProvideRepository,
	ProvideRecorder,
	ProvidePublisher,
	ProvideDispatcher,
	wire.Bind(new(Repository), new(*PostgresRepository)),
	wire.Bind(new(Recorder), new(*OutboxRecorder)),
	wire.Bind(new(Publisher), new(*KafkaPublisher)),
	wire.Bind(new(Dispatcher), new(*OutboxDispatcher)),
)
