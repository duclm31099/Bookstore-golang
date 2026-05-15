package dto

type AddAddressInput struct {
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
}

type UpdateAddressInput struct {
	AddressID      int64
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
}

type DeleteAddressInput struct {
	AddressID int64
	UserID    int64
}
