package service

import (
	"context"
	"strings"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/command"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
)

type AddressService struct {
	address   domain.AddressRepository
	txManager tx.TxManager
}

func NewAddressService(
	address domain.AddressRepository,
	txManager tx.TxManager,
) *AddressService {
	return &AddressService{
		address:   address,
		txManager: txManager,
	}
}

func (s *AddressService) AddAddress(ctx context.Context, cmd command.AddAddressCommand) (int64, error) {

	address := &entity.Address{
		UserID:      cmd.UserID,
		Line1:       strings.TrimSpace(cmd.Line1),
		Line2:       strings.TrimSpace(cmd.Line2),
		Province:    strings.TrimSpace(cmd.ProvinceCode),
		District:    strings.TrimSpace(cmd.DistrictCode),
		Ward:        strings.TrimSpace(cmd.WardCode),
		PostalCode:  strings.TrimSpace(cmd.PostalCode),
		CountryCode: strings.TrimSpace(cmd.CountryCode),
		IsDefault:   cmd.IsDefault,
	}

	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if address.IsDefault {
			if err := s.address.UnsetDefaultByUserID(txCtx, address.UserID); err != nil {
				return err
			}
		}
		return s.address.Insert(txCtx, address)
	})
	if err != nil {
		return 0, err
	}

	return address.ID, nil
}

func (s *AddressService) UpdateAddress(ctx context.Context, cmd command.UpdateAddressCommand) error {
	address, err := s.address.GetByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := address.AssertOwnership(cmd.UserID); err != nil {
		return err
	}

	address.Line1 = strings.TrimSpace(cmd.Line1)
	address.Line2 = strings.TrimSpace(cmd.Line2)
	address.Province = strings.TrimSpace(cmd.ProvinceCode)
	address.District = strings.TrimSpace(cmd.DistrictCode)
	address.Ward = strings.TrimSpace(cmd.WardCode)
	address.PostalCode = strings.TrimSpace(cmd.PostalCode)
	address.CountryCode = strings.TrimSpace(cmd.CountryCode)
	address.IsDefault = cmd.IsDefault

	err = s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if address.IsDefault {
			if err := s.address.UnsetDefaultByUserID(txCtx, address.UserID); err != nil {
				return err
			}
		}
		return s.address.Update(txCtx, address)
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *AddressService) DeleteAddress(ctx context.Context, cmd command.DeleteAddressCommand) error {
	address, err := s.address.GetByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if err := address.AssertOwnership(cmd.UserID); err != nil {
		return err
	}
	return s.address.Delete(ctx, cmd.ID)
}
