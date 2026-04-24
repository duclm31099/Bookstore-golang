// internal/platform/httpx/middleware/request_id.go
package httpx

import (
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return requestid.New()
}
