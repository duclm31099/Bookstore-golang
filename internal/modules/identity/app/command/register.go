package command

type RegisterCommand struct {
	Email    string
	Password string
	FullName string
	Phone    *string
}
