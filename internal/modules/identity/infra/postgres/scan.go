package postgres

import "github.com/jackc/pgx/v5"

func scanUser(row pgx.Row) (*userRow, error) {
	var r userRow
	err := row.Scan(
		&r.ID,
		&r.Email,
		&r.FullName,
		&r.Phone,
		&r.UserType,
		&r.Status,
		&r.EmailVerifiedAt,
		&r.LastLoginAt,
		&r.LockedReason,
		&r.Metadata,
		&r.Version,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanSession(row pgx.Row) (*sessionRow, error) {
	var r sessionRow
	err := row.Scan(
		&r.ID,
		&r.UserID,
		&r.DeviceID,
		&r.RefreshTokenHash,
		&r.SessionStatus,
		&r.ExpiredAt,
		&r.IPAddress,
		&r.UserAgent,
		&r.LastSeenAt,
		&r.RevokedAt,
		&r.RevokedReason,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanDevice(row pgx.Row) (*deviceRow, error) {
	var r deviceRow
	err := row.Scan(
		&r.ID,
		&r.UserID,
		&r.Fingerprint,
		&r.Label,
		&r.FirstSeenAt,
		&r.LastSeenAt,
		&r.RevokedAt,
		&r.RevokedReason,
		&r.Metadata,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanAddress(row pgx.Row) (*addressRow, error) {
	var r addressRow
	err := row.Scan(
		&r.ID,
		&r.UserID,
		&r.RecipientName,
		&r.RecipientPhone,
		&r.Line1,
		&r.Line2,
		&r.Province,
		&r.District,
		&r.Ward,
		&r.PostalCode,
		&r.CountryCode,
		&r.IsDefault,
		&r.Version,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanCredential(row pgx.Row) (*credentialRow, error) {
	var r credentialRow
	err := row.Scan(
		&r.UserID,
		&r.PasswordHash,
		&r.PasswordAlgo,
		&r.PasswordChangedAt,
		&r.FailedLoginCount,
		&r.LastFailedLoginAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
