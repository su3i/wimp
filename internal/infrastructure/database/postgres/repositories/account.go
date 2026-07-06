package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/account"
)

type accountRepository struct {
	db *gorm.DB
}

func (r *accountRepository) Find() (*[]account.Account, error) {
	var _accounts []account.Account

	if err := r.db.Unscoped().Order("created_at DESC").Find(&_accounts).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &_accounts, nil
}

func (r *accountRepository) FindOneByUsername(username string) (*account.Account, error) {
	var _account account.Account

	query := map[string]interface{}{
		"username": username,
	}

	if err := r.db.Unscoped().Where(query).First(&_account).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &_account, nil
}

func (r *accountRepository) Create(payload *account.Account) (*account.Account, error) {
	_account := account.Account{
		Name:          payload.Name,
		Username:      payload.Username,
		Role:          payload.Role,
		InternalRoles: payload.InternalRoles,
		PasswordEnc:   payload.PasswordEnc,
		MFAEnabled:    payload.MFAEnabled,
		MFASecret:     payload.MFASecret,
	}

	err := r.db.Create(&_account).Error

	if err != nil {
		return nil, errors.New("failed to create account: " + err.Error())
	}

	return &_account, nil
}

func (r *accountRepository) Update(payload *account.Account) error {
	err := r.db.Updates(payload).Error

	if err != nil {
		return errors.New("failed to update account: " + err.Error())
	}

	return nil
}

func NewAccountRepository(db *gorm.DB) account.AccountRepository {
	return &accountRepository{db: db}
}
