package command

type AddAddressCommand struct {
	UserID       int64
	Line1        string
	Line2        string
	ProvinceCode string
	DistrictCode string
	WardCode     string
	PostalCode   string
	CountryCode  string
	IsDefault    bool
}
