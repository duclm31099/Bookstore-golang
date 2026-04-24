package httpx

import (
	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, log *zap.Logger) *gin.Engine {
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

	return router
}
