package httpx

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func AuthMiddleware(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "missing authorization header",
				},
			})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid authorization header format",
				},
			})
			return
		}

		tokenString := parts[1]
		claims := jwt.MapClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			log.Error("failed to parse token", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid token",
				},
			})
			return
		}

		if !parsedToken.Valid {
			log.Error("invalid token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "invalid token",
				},
			})
			return
		}
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			c.JSON(401, gin.H{"error": "invalid user ID in token"})
			c.Abort()
			return
		}

		// 5. Convert string sang uuid.UUID
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid UUID format"})
			c.Abort()
			return
		}
		// 6. Set userID vào context ✓ ĐÂY LÀ CHÌA KHÓA
		c.Set("is_authenticated", true)
		c.Set("user_id", userID)

		// Tiếp tục xử lý request
		c.Next()
	}
}
