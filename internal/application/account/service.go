package account

import (
	"errors"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/domain/mfa"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func NewAccount(name string, username string, password string, role account.AccountRole, cfg *config.DatabaseConfig) (*account.Account, error) {
	_accountRepository := database.NewAccountRepository(cfg)

	// Check if username already exists - Fail fast
	_account, err := _accountRepository.FindOneByUsername(username)

	if err != nil || _account != nil {
		return nil, errors.New("Username already registered.")
	}

	// Check password against password requirements
	err = account.CheckPassword(password)

	if err != nil {
		return nil, err
	}

	// Encrypt password
	passwordEnc, err := account.EncryptPassword(password)

	if err != nil {
		return nil, err
	}

	mfaSecret, err := mfa.GenerateMFASecret()

	if err != nil {
		return nil, err
	}

	internalRoleKey := account.BuildRoleKey("default", authorization.AuthorizationDomainOrg, string(role))

	internalRoleJson := map[string]string{
		"default": internalRoleKey,
	}

	// Create account
	_account = &account.Account{
		Name:          name,
		Username:      username,
		Role:          role,
		InternalRoles: internalRoleJson,
		PasswordEnc:   passwordEnc,
		MFAEnabled:    false,
		MFASecret:     mfaSecret,
	}

	return _accountRepository.Create(_account)
}

func RetrieveAccounts(cfg *config.DatabaseConfig) (*[]account.Account, error) {
	_accountRepository := database.NewAccountRepository(cfg)

	return _accountRepository.Find()
}

func RetrieveAccount(username string, cfg *config.DatabaseConfig) (*account.Account, error) {
	_accountRepository := database.NewAccountRepository(cfg)

	return _accountRepository.FindOneByUsername(username)
}

func RetrieveAccountWithPassword(username string, password string, cfg *config.DatabaseConfig) (*account.Account, error) {
	_accountRepository := database.NewAccountRepository(cfg)

	_account, err := _accountRepository.FindOneByUsername(username)

	if err != nil {
		return nil, err
	}

	if _account == nil {
		return nil, errors.New("Invalid account.")
	}

	err = account.VerifyPassword(_account.PasswordEnc, password)

	if err != nil {
		return nil, errors.New("Invalid password.")
	}

	return _account, nil
}

func EnableTOTP(username string, cfg *config.DatabaseConfig) error {
	_accountRepository := database.NewAccountRepository(cfg)

	_account, err := _accountRepository.FindOneByUsername(username)

	if err != nil {
		return err
	}

	if _account == nil {
		return errors.New("Invalid account.")
	}

	_account.MFAEnabled = true

	return _accountRepository.Update(_account)
}

func UpdateAccount(oldUsername string, name *string, username *string, cfg *config.DatabaseConfig) (*account.Account, error) {
	_accountRepository := database.NewAccountRepository(cfg)

	// Check if username exists - Fail fast
	_account, err := _accountRepository.FindOneByUsername(oldUsername)

	if err != nil || _account == nil {
		return nil, errors.New("Invalid account.")
	}

	if name != nil {
		_account.Name = *name
	}

	if username != nil {
		_account.Username = *username
	}

	// Save updated account
	if err := _accountRepository.Update(_account); err != nil {
		return nil, err
	}

	return _account, nil
}
