package http

import "github.com/gin-gonic/gin"

func RegisterRoutes(
	r *gin.Engine,
	authHandler *AuthHandler,
	profileHandler *ProfileHandler,
	addressHandler *AddressHandler,
	authMiddleware gin.HandlerFunc,
	idempotencyMiddleware gin.HandlerFunc,
) {
	api := r.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST("/register", idempotencyMiddleware, authHandler.Register)
		auth.POST("/login", idempotencyMiddleware, authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/verify-email", idempotencyMiddleware, authHandler.VerifyEmail)
		auth.POST("/logout", authMiddleware, authHandler.Logout)
	}

	me := api.Group("/me")
	me.Use(authMiddleware)
	{
		me.GET("", profileHandler.GetMe)
		me.GET("/sessions", profileHandler.ListSessions)
		me.DELETE("/sessions", profileHandler.RevokeAllSessions)

		me.GET("/devices", profileHandler.ListDevices)
		me.DELETE("/devices/:deviceId", profileHandler.RevokeDevice)

		me.GET("/addresses", addressHandler.ListAddresses)
		me.POST("/addresses", addressHandler.AddAddress)
		me.PATCH("/addresses/:addressId", addressHandler.UpdateAddress)
		me.DELETE("/addresses/:addressId", addressHandler.DeleteAddress)
	}
}
