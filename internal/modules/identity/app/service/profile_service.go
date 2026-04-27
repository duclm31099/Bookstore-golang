package service

import (
	"context"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/query"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
)

type ProfileService struct {
	txManager tx.TxManager
	user      domain.UserRepository
	devices   domain.DeviceRepository
	queryRepo query.QueryRepository
	sessions  domain.SessionRepository
	clock     ports.Clock
}

func NewProfileService(
	txManager tx.TxManager,
	userRepo domain.UserRepository,
	deviceRepo domain.DeviceRepository,
	queryRepo query.QueryRepository,
	sessions domain.SessionRepository,
	clock ports.Clock,
) *ProfileService {
	return &ProfileService{
		txManager: txManager,
		user:      userRepo,
		devices:   deviceRepo,
		queryRepo: queryRepo,
		sessions:  sessions,
		clock:     clock,
	}
}

func (s *ProfileService) GetMe(ctx context.Context, userID int64) (*query.MeView, error) {
	return s.queryRepo.GetMe(ctx, userID)
}

func (s *ProfileService) ListSessions(ctx context.Context, userID int64) ([]*query.SessionView, error) {
	return s.queryRepo.ListSessions(ctx, userID)
}

func (s *ProfileService) ListDevices(ctx context.Context, userID int64) ([]*query.DeviceView, error) {
	return s.queryRepo.ListDevices(ctx, userID)
}

func (s *ProfileService) ListAddresses(ctx context.Context, userID int64) ([]*query.AddressView, error) {
	return s.queryRepo.ListAddresses(ctx, userID)
}
func (s *ProfileService) RevokeAllSessions(ctx context.Context, userID int64) error {
	return s.sessions.RevokeAllByUserID(ctx, userID, s.clock.Now())
}
func (s *ProfileService) RevokeDevice(ctx context.Context, userID, deviceID int64) error {
	device, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}
	if err := device.AssertOwnership(userID); err != nil {
		return err
	}

	now := s.clock.Now()

	return s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.devices.Revoke(txCtx, deviceID, now); err != nil {
			return err
		}
		return s.sessions.RevokeAllByUserID(txCtx, userID, now)
	})
}
