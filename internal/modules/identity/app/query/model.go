package query

import "time"

type MeView struct {
	UserID          int64
	Email           string
	FullName        string
	Status          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
}

type SessionView struct {
	ID        int64
	DeviceID  int64
	IPAddress *string
	UserAgent string
	ExpiredAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type DeviceView struct {
	ID          int64
	Fingerprint string
	Label       string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
}

type AddressView struct {
	ID             int64
	RecipientName  string
	RecipientPhone string
	Province       string
	District       string
	Ward           string
	Line1          string
	Line2          string
	IsDefault      bool
}
