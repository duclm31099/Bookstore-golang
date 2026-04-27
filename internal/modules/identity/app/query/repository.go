package query

import "context"

type QueryRepository interface {
	GetMe(ctx context.Context, userID int64) (*MeView, error)
	ListSessions(ctx context.Context, userID int64) ([]*SessionView, error)
	ListDevices(ctx context.Context, userID int64) ([]*DeviceView, error)
	ListAddresses(ctx context.Context, userID int64) ([]*AddressView, error)
}
