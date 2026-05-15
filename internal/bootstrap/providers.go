// internal/bootstrap/providers.go
package bootstrap

import (
	"net/http"
	"strings"
	"time"

	identity_service "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/service"
	identity_http "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http"
	identity_middleware "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	identity_adapters "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/infra/adapters"
	identity_postgres "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/infra/postgres"
	auth "github.com/duclm99/bookstore-backend-v2/internal/platform/auth"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	db "github.com/duclm99/bookstore-backend-v2/internal/platform/db"
	httpx "github.com/duclm99/bookstore-backend-v2/internal/platform/httpx"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/idempotency"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/kafka"
	logger "github.com/duclm99/bookstore-backend-v2/internal/platform/logger"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/observability"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/outbox"
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
	// Register logger globally for use in middleware, handlers, etc.
	// This allows functions anywhere in the package to call log.Info, log.Error, etc.
	// without needing to pass the logger explicitly.
	zap.ReplaceGlobals(log)

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

func ProvideKafkaConfig(cfg *config.Config) kafka.Config {
	kafkaCfg := kafka.DefaultConfig()
	if len(cfg.Kafka.Brokers) > 0 {
		kafkaCfg.Brokers = cfg.Kafka.Brokers
	}
	if cfg.Kafka.ClientID != "" {
		kafkaCfg.ClientID = cfg.Kafka.ClientID
	}
	if cfg.Kafka.ConsumerGroupID != "" {
		kafkaCfg.ConsumerGroupID = cfg.Kafka.ConsumerGroupID
	}
	return kafkaCfg
}

func ProvideIdempotencyDBTX(pool *pgxpool.Pool) idempotency.DBTX {
	return pool
}

func ProvideUniversalRedisClient(client *goredis.Client) goredis.UniversalClient {
	return client
}

func ProvideGinEngine(
	cfg *config.Config,
	log *zap.Logger,
	authHandler *identity_http.AuthHandler,
	profileHandler *identity_http.ProfileHandler,
	addressHandler *identity_http.AddressHandler,
	authMiddleware identity_middleware.AuthMiddleware,
	idempotencySvc idempotency.Service,
	strictAuthMiddleware identity_middleware.StrictAuthMiddleware,
) *gin.Engine {
	engine := httpx.NewRouter(cfg, log)

	idempotencyMiddleware := idempotency.GinMiddleware(idempotencySvc, idempotency.MiddlewareConfig{
		KeyHeader: "Idempotency-Key",
		ScopeResolver: func(c *gin.Context) string {
			arr := strings.Split(c.FullPath(), "/")
			action := arr[len(arr)-1]
			log.Info("middleware key", zap.String("key", "identity:"+action))
			return "identity:" + action
		},
	})

	// Nhúng router identity
	identity_http.RegisterRoutes(engine, authHandler, profileHandler, addressHandler, authMiddleware, idempotencyMiddleware, strictAuthMiddleware)

	return engine
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
	ProvideUniversalRedisClient,
	ProvideAuthManager,
	ProvideHealthHandler,
	ProvideKafkaConfig,
	kafka.ProviderSet,
	ProvideIdempotencyDBTX,
	idempotency.ProviderSet,
	outbox.ProviderSet,
)

var HTTPSet = wire.NewSet(
	ProvideGinEngine,
	ProvideHTTPServer,
	ProvideShutdownTimeout,
)

var IdentityModuleSet = wire.NewSet(
	identity_postgres.ProviderSet,
	identity_adapters.ProviderSet,
	identity_service.ProviderSet,
	identity_http.ProviderSet, // Thêm provider set của HTTP layer

	// Interface bindings
	wire.Bind(new(identity_http.AuthUseCase), new(*identity_service.AuthService)),
	wire.Bind(new(identity_http.ProfileUseCase), new(*identity_service.ProfileService)),
	wire.Bind(new(identity_http.AddressUseCase), new(*identity_service.AddressService)),
	wire.Bind(new(identity_http.AddressQueryUseCase), new(*identity_service.ProfileService)),
)

// ModuleSet gom tất cả infrastructure và service của mọi module
var ModuleSet = wire.NewSet(
	// Identity Module Set
	IdentityModuleSet,
)

var APISet = wire.NewSet(
	PlatformSet, // Everything Infrastructure
	HTTPSet,     // Gin Server router
	ModuleSet,   // Module Business
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
