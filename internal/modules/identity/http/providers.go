package http

import (
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/http/middleware"
	"github.com/google/wire"
)

// ProviderSet chứa tất cả dependencies của phần HTTP (Identity module)
var ProviderSet = wire.NewSet(
	// 1. HTTP Handlers
	NewAuthHandler,
	NewProfileHandler,
	NewAddressHandler,

	// 2. Middlewares
	middleware.NewAuthMiddleware,
	middleware.NewStrictAuthMiddleware,
)
