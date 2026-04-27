package policy

import (
	entity "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
)

type DevicePolicy struct {
	MaxDevicesPerUser int
	MaxSessionTTL     int
}

func NewDevicePolicy() *DevicePolicy {
	return &DevicePolicy{
		MaxDevicesPerUser: 5,
		MaxSessionTTL:     30,
	}
}

// CanRegisterNewDevice kiểm tra user có thể thêm device mới không
// activeDevices là danh sách device chưa bị revoke
func (p *DevicePolicy) CanRegisterNewDevice(activeDevices []*entity.Device) error {
	if len(activeDevices) >= p.MaxDevicesPerUser {
		return err.ErrDeviceLimitReached
	}
	return nil
}
