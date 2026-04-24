// internal/bootstrap/providers.go
package bootstrap

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	platformdb "github.com/duclm99/bookstore-backend-v2/internal/platform/db"
	platformhttpx "github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	platformlogger "github.com/duclm99/bookstore-backend-v2/internal/platform/logger"
	platformredis "github.com/duclm99/bookstore-backend-v2/internal/platform/redis"
	platformtx "github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
)

func ProvideConfig() *config.Config {
	return config.MustLoad()
}

func ProvideLogger(cfg *config.Config) (*zap.Logger, func(), error) {
	log, err := platformlogger.New(cfg.Logger)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = log.Sync()
	}

	return log, cleanup, nil
}

func ProvideDBPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
	pool, err := platformdb.NewPool(cfg.DB)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup, nil
}

func ProvideTxManager(pool *pgxpool.Pool) *platformtx.Manager {
	return platformtx.NewManager(pool)
}

func ProvideRedis(cfg *config.Config) (*goredis.Client, func(), error) {
	rdb, err := platformredis.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = rdb.Close()
	}

	return rdb, cleanup, nil
}

func ProvideGinEngine(cfg *config.Config, log *zap.Logger) *gin.Engine {
	return platformhttpx.NewRouter(cfg.App, log)
}

func ProvideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.App.Port),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func ProvideShutdownTimeout(cfg *config.Config) time.Duration {
	return cfg.App.GracefulShutdownTimeout
}

func ProvideAPIApp(
	server *http.Server,
	logger *zap.Logger,
	shutdownTimeout time.Duration,
) *APIApp {
	return NewAPIApp(server, logger, shutdownTimeout)
}

var PlatformSet = wire.NewSet(
	ProvideConfig,
	ProvideLogger,
	ProvideDBPool,
	ProvideTxManager,
	ProvideRedis,
)

var HTTPSet = wire.NewSet(
	ProvideGinEngine,
	ProvideHTTPServer,
	ProvideShutdownTimeout,
)

var APISet = wire.NewSet(
	PlatformSet,
	HTTPSet,
	ProvideAPIApp,
)

// var WorkerRuntimeSet = wire.NewSet(
// 	ProvideOutboxPublisher,
// 	ProvideConsumerRegistry,
// 	ProvideWorkerApp,
// )
// var SchedulerRuntimeSet = wire.NewSet(
// 	ProvideJobRegistry,
// 	ProvideSchedulerRunner,
// 	ProvideSchedulerApp,
// )

// var WorkerSet = wire.NewSet(
// 	PlatformSet,
// 	ModuleConsumerSet,
// 	WorkerRuntimeSet,
// )

// var SchedulerSet = wire.NewSet(
// 	PlatformSet,
// 	ModuleJobSet,
// 	SchedulerRuntimeSet,
// )
