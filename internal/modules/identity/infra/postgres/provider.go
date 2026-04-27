package postgres

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserRepository,
	NewCredentialRepository,
	NewSessionRepository,
	NewDeviceRepository,
	NewAddressRepository,
	NewQueryRepository,
)
