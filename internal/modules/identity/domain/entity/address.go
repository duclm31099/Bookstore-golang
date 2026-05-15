package entity

import (
	"time"

	err "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/error"
)

// Address là shipping address của user
// Được dùng trong checkout để snapshot địa chỉ giao hàng
// Sau khi order đã tạo, address snapshot trong order không đổi
// dù user xóa address này đi
type Address struct {
	ID             int64
	UserID         int64
	RecipientName  string
	RecipientPhone string
	Line1          string
	Line2          string
	Province       string
	District       string
	Ward           string
	PostalCode     string
	CountryCode    string
	IsDefault      bool
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AssertOwnership đảm bảo chỉ owner mới được mutate address
// Đây là enforcement boundary quan trọng — không để handler tự check
func (a *Address) AssertOwnership(userID int64) error {
	if a.UserID != userID {
		return err.ErrAddressNotOwned
	}
	return nil
}
