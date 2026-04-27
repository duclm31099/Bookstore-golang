package httpx

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/observability"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, log *zap.Logger, healthHandler *observability.HealthHandler) *gin.Engine {
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

	// Register observability routes
	if healthHandler != nil {
		healthHandler.RegisterRoutes(router)
	}

	return router
}
