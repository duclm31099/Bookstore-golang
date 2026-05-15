package postgres

import (
	"fmt"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/query"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	valueobject "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
)

func mapUserRowToEntity(r *userRow) (*entity.User, error) {
	email, err := valueobject.NewEmail(r.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid email in users row id=%d: %w", r.ID, err)
	}

	status := valueobject.UserStatus(r.Status)

	return &entity.User{
		ID:              r.ID,
		Email:           email,
		Phone:           r.Phone,
		UserType:        r.UserType,
		FullName:        r.FullName,
		Status:          status,
		EmailVerifiedAt: r.EmailVerifiedAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}, nil
}

func mapCredentialRowToEntity(r *credentialRow) *entity.Credential {
	return &entity.Credential{
		UserID:            r.UserID,
		PasswordHash:      r.PasswordHash,
		PasswordAlgo:      r.PasswordAlgo,
		PasswordChangedAt: r.PasswordChangedAt,
		FailedLoginCount:  r.FailedLoginCount,
		LastFailedLoginAt: r.LastFailedLoginAt,
	}
}

func mapSessionRowToEntity(r *sessionRow) *entity.Session {
	return &entity.Session{
		ID:               r.ID,
		UserID:           r.UserID,
		DeviceID:         r.DeviceID,
		RefreshTokenHash: r.RefreshTokenHash,
		SessionStatus:    r.SessionStatus,
		ExpiredAt:        r.ExpiredAt,
		IPAddress:        r.IPAddress,
		UserAgent:        r.UserAgent,
		LastSeenAt:       r.LastSeenAt,
		RevokedAt:        r.RevokedAt,
		RevokedReason:    r.RevokedReason,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

func mapSessionRowToView(r *sessionRow) *query.SessionView {
	return &query.SessionView{
		ID:        r.ID,
		DeviceID:  r.DeviceID,
		ExpiredAt: r.ExpiredAt,
		IPAddress: r.IPAddress,
		UserAgent: r.UserAgent,
		RevokedAt: r.RevokedAt,
		CreatedAt: r.CreatedAt,
	}
}

func mapAddressRowToEntity(r *addressRow) *entity.Address {
	return &entity.Address{
		ID:             r.ID,
		UserID:         r.UserID,
		RecipientName:  r.RecipientName,
		RecipientPhone: r.RecipientPhone,
		Line1:          r.Line1,
		Line2:          r.Line2,
		Province:       r.Province,
		District:       r.District,
		Ward:           r.Ward,
		PostalCode:     r.PostalCode,
		CountryCode:    r.CountryCode,
		IsDefault:      r.IsDefault,
		Version:        r.Version,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func mapAddressRowToView(r *addressRow) *query.AddressView {
	return &query.AddressView{
		ID:             r.ID,
		RecipientName:  r.RecipientName,
		RecipientPhone: r.RecipientPhone,
		Line1:          r.Line1,
		Line2:          r.Line2,
		Province:       r.Province,
		District:       r.District,
		Ward:           r.Ward,
		IsDefault:      r.IsDefault,
	}
}

func mapUserRowToMeView(r *userRow) *query.MeView {
	return &query.MeView{
		UserID:          r.ID,
		Email:           r.Email,
		FullName:        r.FullName,
		Status:          r.Status,
		EmailVerifiedAt: r.EmailVerifiedAt,
		CreatedAt:       r.CreatedAt,
	}
}

func mapDeviceRowToEntity(r *deviceRow) *entity.Device {
	return &entity.Device{
		ID:            r.ID,
		UserID:        r.UserID,
		Fingerprint:   r.Fingerprint,
		Label:         r.Label,
		FirstSeenAt:   r.FirstSeenAt,
		LastSeenAt:    r.LastSeenAt,
		RevokedAt:     r.RevokedAt,
		RevokedReason: r.RevokedReason,
		Metadata:      r.Metadata,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

func mapDeviceRowToView(r *deviceRow) *query.DeviceView {
	return &query.DeviceView{
		ID:          r.ID,
		Fingerprint: r.Fingerprint,
		Label:       r.Label,
		FirstSeenAt: r.FirstSeenAt,
		LastSeenAt:  r.LastSeenAt,
		RevokedAt:   r.RevokedAt,
	}
}
