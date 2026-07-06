package account

type AccountRepository interface {
	Find() (*[]Account, error)
	FindOneByUsername(username string) (*Account, error)
	Create(payload *Account) (*Account, error)
	Update(payload *Account) error
}