package postgres

import "time"

// rows.go là lớp trung gian thuần DB shape, tách khỏi domain entity.
// Blueprint đã gợi ý rõ mapper.go cho DB row <-> domain model,
// và việc có row structs riêng sẽ giúp bạn cô lập nullability, column order và kiểu lưu trữ của DB

// User row
type userRow struct {
	ID              int64
	Email           string
	FullName        string
	Phone           *string
	UserType        string
	Status          string
	EmailVerifiedAt *time.Time
	LastLoginAt     *time.Time
	LockedReason    *string
	Metadata        map[string]interface{}
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Session row
type sessionRow struct {
	ID               int64
	UserID           int64
	DeviceID         *int64
	RefreshTokenHash string
	SessionStatus    string
	ExpiredAt        time.Time
	IPAddress        string
	UserAgent        string
	LastSeenAt       time.Time
	RevokedAt        *time.Time
	RevokedReason    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Device row
type deviceRow struct {
	ID            int64
	UserID        int64
	Fingerprint   string
	Label         string
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	RevokedAt     *time.Time
	RevokedReason string
	Metadata      map[string]interface{}
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Address row
type addressRow struct {
	ID          int64
	UserID      int64
	Line1       string
	Line2       string
	Province    string
	District    string
	Ward        string
	PostalCode  string
	CountryCode string
	IsDefault   bool
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Credential row
type credentialRow struct {
	UserID            int64
	PasswordHash      string
	PasswordAlgo      string
	PasswordChangedAt time.Time
	FailedLoginCount  int
	LastFailedLoginAt *time.Time
}
