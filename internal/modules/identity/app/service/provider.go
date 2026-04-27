package service

import (
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/policy"
	"github.com/google/wire"
)

// ProvidePolicies khởi tạo các domain policy với default values.
func ProvideRegisterPolicy() policy.RegisterPolicy {
	return *policy.NewRegisterPolicy()
}

func ProvideDevicePolicy() policy.DevicePolicy {
	return *policy.NewDevicePolicy()
}

// ProviderSet gom AuthService và các domain policy để wire inject.
var ProviderSet = wire.NewSet(
	NewAuthService,
	NewProfileService,
	NewAddressService,
	ProvideRegisterPolicy,
	ProvideDevicePolicy,
)
