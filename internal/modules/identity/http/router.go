package http

import (
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	r *gin.Engine,
	authHandler *AuthHandler,
	profileHandler *ProfileHandler,
	addressHandler *AddressHandler,
	authMiddleware middleware.AuthMiddleware,
	idempotencyMiddleware gin.HandlerFunc,
	strictAuthMiddleware middleware.StrictAuthMiddleware,
) {
	api := r.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST("/register", idempotencyMiddleware, authHandler.Register)
		auth.POST("/login", idempotencyMiddleware, authHandler.Login)
		auth.POST("/refresh-token", gin.HandlerFunc(authMiddleware), idempotencyMiddleware, authHandler.RefreshToken)
		auth.POST("/verify-email", idempotencyMiddleware, authHandler.VerifyEmail)
		auth.POST("/logout", gin.HandlerFunc(authMiddleware), idempotencyMiddleware, authHandler.Logout)
	}

	me := api.Group("/me")
	me.Use(gin.HandlerFunc(authMiddleware))
	{
		me.GET("", profileHandler.GetMe)
		me.GET("/sessions", profileHandler.ListSessions)
		me.DELETE("/sessions", profileHandler.RevokeAllSessions)
		me.POST("/change-password", gin.HandlerFunc(strictAuthMiddleware), idempotencyMiddleware, authHandler.ChangePassword)

		me.GET("/devices", profileHandler.ListDevices)
		me.DELETE("/devices/:deviceId", profileHandler.RevokeDevice)

		me.GET("/addresses", addressHandler.ListAddresses)
		me.POST("/addresses", addressHandler.AddAddress)
		me.PATCH("/addresses/:addressId", addressHandler.UpdateAddress)
		me.DELETE("/addresses/:addressId", addressHandler.DeleteAddress)
	}
}
