package httpx

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(cfg config.AppConfig, log *zap.Logger) *gin.Engine {
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.Use(RecoveryMiddleware(log))
	router.Use(LoggerMiddleware(log))

	return router
}
