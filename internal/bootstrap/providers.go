// internal/bootstrap/providers.go
package bootstrap

import (
	"net/http"
	"time"

	identity_adapters "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/infra/adapters"
	identity_postgres "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/infra/postgres"
	identity_service "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/service"
	auth "github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	db "github.com/duclm99/bookstore-backend-v2/internal/platform/db"
	httpx "github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	logger "github.com/duclm99/bookstore-backend-v2/internal/platform/logger"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/observability"
	redis "github.com/duclm99/bookstore-backend-v2/internal/platform/redis"
	tx "github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func ProvideConfig() *config.Config {
	return config.MustLoad()
}

func ProvideLogger(cfg *config.Config) (*zap.Logger, func(), error) {
	log, err := logger.New(cfg.Logger)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = log.Sync()
	}

	return log, cleanup, nil
}

func ProvideDBPool(cfg *config.Config) (*pgxpool.Pool, func(), error) {
	pool, err := db.NewPool(cfg.DB)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		pool.Close()
	}

	return pool, cleanup, nil
}

func ProvideTxManager(pool *pgxpool.Pool) *tx.Manager {
	return tx.NewPoolManager(pool)
}

// ProvideTxManagerInterface bind *tx.Manager vào interface tx.TxManager
// để wire có thể inject đúng type vào service.
func ProvideTxManagerInterface(m *tx.Manager) tx.TxManager {
	return m
}
func ProvideAuthManager(cfg *config.Config) *auth.Auth {
	return auth.NewAuthManager(cfg.JWT)
}
func ProvideRedis(cfg *config.Config) (*goredis.Client, func(), error) {
	rdb, err := redis.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = rdb.Close()
	}

	return rdb, cleanup, nil
}

func ProvideHealthHandler(cfg *config.Config, redisClient *goredis.Client) *observability.HealthHandler {
	return observability.NewHealthHandler(redisClient, cfg)
}

func ProvideGinEngine(cfg *config.Config, log *zap.Logger, healthHandler *observability.HealthHandler) *gin.Engine {
	return httpx.NewRouter(cfg, log, healthHandler)
}

func ProvideHTTPServer(cfg *config.Config, engine *gin.Engine) *http.Server {
	return httpx.NewServer(engine, cfg.App.Port)
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
	ProvideTxManagerInterface,
	ProvideRedis,
	ProvideAuthManager,
	ProvideHealthHandler,
)

var HTTPSet = wire.NewSet(
	ProvideGinEngine,
	ProvideHTTPServer,
	ProvideShutdownTimeout,
)

var APISet = wire.NewSet(
	PlatformSet,
	HTTPSet,
	ModuleSet,
	ProvideAPIApp,
)

// ModuleSet gom tất cả infrastructure và service của mọi module
var ModuleSet = wire.NewSet(
	// Identity: postgres repositories
	identity_postgres.ProviderSet,
	// Identity: port adapters (hasher, token, verify, event, clock)
	identity_adapters.ProviderSet,
	// Identity: application services
	identity_service.ProviderSet,

	// Nơi đây sẽ là bến đỗ cho mọi module sau này:
	// catalog_postgres.ProviderSet,
	// order_postgres.ProviderSet,
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
