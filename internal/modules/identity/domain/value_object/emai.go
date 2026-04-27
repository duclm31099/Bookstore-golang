package valueobject

import (
	"errors"
	"net/mail"
	"strings"
)

type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if _, err := mail.ParseAddress(normalized); err != nil {
		return Email{}, errors.New("invalid email format")
	}
	if len(normalized) > 255 {
		return Email{}, errors.New("email too long")
	}
	return Email{value: normalized}, nil
}

func MustEmail(raw string) Email {
	email, err := NewEmail(raw)
	if err != nil {
		panic("invalid email: " + raw)
	}
	return email
}

func (e Email) String() string          { return e.value }
func (e Email) IsZero() bool            { return e.value == "" }
func (e Email) Equals(other Email) bool { return e.value == other.value }
