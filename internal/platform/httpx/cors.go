package httpx

import (
	"strconv"
	"strings"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		for _, allowed := range cfg.AllowedOrigins {
			if strings.TrimSpace(allowed) == origin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
				c.Writer.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))
				c.Writer.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(cfg.AllowCredentials))
				break
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
