package httpx

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, log *zap.Logger, rdb *goredis.Client) *gin.Engine {
	if cfg.App.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(RecoveryMiddleware(log))
	router.Use(LoggerMiddleware(log))
	router.Use(CORSMiddleware(cfg))
	router.Use(RateLimitMiddleware(rdb, cfg.RateLimit, log))

	return router
}
