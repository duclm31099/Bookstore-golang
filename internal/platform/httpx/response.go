// internal/platform/httpx/response.go
package httpx

import "github.com/gin-gonic/gin"

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    data,
	})
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
