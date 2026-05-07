package domain

import (
	"context"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/entity"
	object "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/value_object"
)

// DBTX là abstraction cho phép repo chạy trên cả pool lẫn transaction
// Application service truyền tx vào đây khi cần atomic mutation
// Đây là pattern quan trọng để transaction boundary ở đúng app layer
type DBTX interface {
}

// UserRepository — chỉ chứa operations mà domain/application thực sự cần
// Không có generic CRUD, mỗi method có semantic nghiệp vụ rõ ràng
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*entity.User, error)
	GetByEmail(ctx context.Context, email object.Email) (*entity.User, error)
	ExistsByEmail(ctx context.Context, email object.Email) (bool, error)
	Insert(ctx context.Context, user *entity.User) error
	UpdateStatus(ctx context.Context, id int64, status object.UserStatus) error
	MarkEmailVerified(ctx context.Context, id int64, verifiedAt time.Time) error
}

// CredentialRepository tách biệt khỏi UserRepository vì credential
// là sensitive aggregate cần được access-control riêng
type CredentialRepository interface {
	GetByUserID(ctx context.Context, userID int64) (*entity.Credential, error)
	Insert(ctx context.Context, cred *entity.Credential) error
	UpdatePasswordHash(ctx context.Context, userID int64, hash string, changedAt time.Time) error
}

// SessionRepository
type SessionRepository interface {
	Update(ctx context.Context, session *entity.Session) error
	Insert(ctx context.Context, session *entity.Session) error
	GetByRefreshTokenHash(ctx context.Context, hash string) (*entity.Session, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*entity.Session, error)
	Revoke(ctx context.Context, id int64, userID int64, revokedAt time.Time) error
	GetByRefreshTokenHashForUpdate(ctx context.Context, hash string) (*entity.Session, error)
	RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error

	GetByDeviceID(ctx context.Context, userID int64, deviceID int64) (*entity.Session, error)
	Upsert(ctx context.Context, session *entity.Session) error
}

// DeviceRepository
type DeviceRepository interface {
	GetByID(ctx context.Context, id int64) (*entity.Device, error)
	GetByFingerprint(ctx context.Context, userID int64, fingerprint string) (*entity.Device, error)
	ListActiveByUserID(ctx context.Context, userID int64) ([]*entity.Device, error)
	Upsert(ctx context.Context, device *entity.Device) error
	Revoke(ctx context.Context, id int64, revokedAt time.Time) error
}

// AddressRepository
type AddressRepository interface {
	GetByID(ctx context.Context, id int64) (*entity.Address, error)
	ListByUserID(ctx context.Context, userID int64) ([]*entity.Address, error)
	Insert(ctx context.Context, address *entity.Address) error
	Update(ctx context.Context, address *entity.Address) error
	Delete(ctx context.Context, id int64) error
	UnsetDefaultByUserID(ctx context.Context, userID int64) error
}
