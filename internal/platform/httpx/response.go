// internal/platform/httpx/response.go
package httpx

import (
	"github.com/gin-gonic/gin"
)

type SuccessResponse struct {
	Success bool        `json:"success"`        // Always true for success
	Message string      `json:"message"`        // Human-readable message
	Data    interface{} `json:"data,omitempty"` // Payload (nullable)
	Code    int         `json:"code"`
}

// Error response structure
// Cấu trúc JSON chuẩn cho error response
type ErrorResponse struct {
	Success bool        `json:"success"`         // Always false for errors
	Message string      `json:"message"`         // User-friendly error message
	Error   interface{} `json:"error,omitempty"` // Technical error details (nullable)
	Code    int         `json:"code"`
}

// Success gửi success response
// Sử dụng trong handlers: response.Success(c, 200, "OK", data)
func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
		Code:    statusCode,
	})
}

// Error gửi error response
// Sử dụng trong handlers: response.Error(c, 400, "Bad request", err)
func Error(c *gin.Context, statusCode int, message string, err interface{}) {
	// Abort: stop execution chain (không chạy handlers tiếp theo)
	c.AbortWithStatusJSON(statusCode, ErrorResponse{
		Success: false,
		Message: message,
		Error:   err,
		Code:    statusCode,
	})
}
