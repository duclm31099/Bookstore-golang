package postgres

import (
	"context"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain"
	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	err_domain "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/tx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AddressRepository struct {
	BaseRepository
}

func NewAddressRepository(pool *pgxpool.Pool) domain.AddressRepository {
	return &AddressRepository{
		BaseRepository: NewBaseRepository(pool),
	}
}

const (
	queryGetAddressByID = `
		SELECT id, user_id, recipient_name, recipient_phone,
			address_line1, address_line2, province_code,
			district_code, ward_code, postal_code, country_code,
			is_default, version, created_at, updated_at
		FROM addresses
		WHERE id = $1
	`

	queryListAddressByUserID = `
		SELECT id, user_id, recipient_name, recipient_phone,
			address_line1, address_line2, province_code,
			district_code, ward_code, postal_code, country_code,
			is_default, version, created_at, updated_at
		FROM addresses
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`

	queryInsertAddress = `
		INSERT INTO addresses (
			user_id, recipient_name, recipient_phone,
			address_line1, address_line2, province_code,
			district_code, ward_code, postal_code, country_code, is_default
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	queryUpdateAddress = `
		UPDATE addresses
		SET recipient_name = $2, recipient_phone = $3,
			address_line1 = $4, address_line2 = $5,
			province_code = $6, district_code = $7, ward_code = $8,
			postal_code = $9, country_code = $10, is_default = $11
		WHERE id = $1
	`

	queryDeleteAddress = `
		DELETE FROM addresses
		WHERE id = $1
	`

	queryUnsetDefaultByUserID = `
		UPDATE addresses
		SET is_default = false
		WHERE user_id = $1
	`
)

func (r *AddressRepository) GetByID(ctx context.Context, id int64) (*entity.Address, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	row := executor.QueryRow(ctx, queryGetAddressByID, id)

	addressRow, err := scanAddress(row)
	if err != nil {
		if isNoRows(err) {
			return nil, err_domain.ErrAddressNotFound
		}
		return nil, err
	}

	return mapAddressRowToEntity(addressRow), nil

}

func (r *AddressRepository) ListByUserID(ctx context.Context, userID int64) ([]*entity.Address, error) {
	executor := tx.GetExecutor(ctx, r.pool)

	rows, err := executor.Query(ctx, queryListAddressByUserID, userID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []*entity.Address
	for rows.Next() {
		row, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, mapAddressRowToEntity(row))
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return addresses, nil
}

func (r *AddressRepository) Insert(ctx context.Context, address *entity.Address) error {
	executor := tx.GetExecutor(ctx, r.pool)

	err := executor.QueryRow(ctx, queryInsertAddress,
		address.UserID, address.RecipientName, address.RecipientPhone,
		address.Line1, address.Line2, address.Province,
		address.District, address.Ward, address.PostalCode, address.CountryCode,
		address.IsDefault,
	).Scan(&address.ID)
	return err
}

func (r *AddressRepository) Update(ctx context.Context, address *entity.Address) error {
	executor := tx.GetExecutor(ctx, r.pool)

	tag, err := executor.Exec(ctx, queryUpdateAddress,
		address.ID, address.RecipientName, address.RecipientPhone,
		address.Line1, address.Line2, address.Province,
		address.District, address.Ward, address.PostalCode, address.CountryCode,
		address.IsDefault,
	)
	if tag.RowsAffected() == 0 {
		return err_domain.ErrAddressNotFound
	}
	return err
}
func (r *AddressRepository) Delete(ctx context.Context, id int64) error {
	executor := tx.GetExecutor(ctx, r.pool)

	tag, err := executor.Exec(ctx, queryDeleteAddress, id)
	if tag.RowsAffected() == 0 {
		return err_domain.ErrAddressNotFound
	}
	return err

}

func (r *AddressRepository) UnsetDefaultByUserID(ctx context.Context, userID int64) error {
	executor := tx.GetExecutor(ctx, r.pool)

	_, err := executor.Exec(ctx, queryUnsetDefaultByUserID, userID)
	return err
}
