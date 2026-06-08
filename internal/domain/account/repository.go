package account

type AccountRepository interface {
	Find() (*[]Account, error)
	FindOneByEmail(email string) (*Account, error)
	Create(payload *Account) (*Account, error)
	Update(payload *Account) error
}