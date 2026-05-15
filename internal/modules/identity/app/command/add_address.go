package command

type AddAddressCommand struct {
	UserID         int64
	RecipientName  string
	RecipientPhone string
	Line1          string
	Line2          string
	ProvinceCode   string
	DistrictCode   string
	WardCode       string
	PostalCode     string
	CountryCode    string
	IsDefault      bool
}
