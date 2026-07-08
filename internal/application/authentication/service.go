package authentication

import (
	"errors"
	"fmt"
	"time"

	"github.com/su3i/wimp/internal/application/account"
	"github.com/su3i/wimp/internal/config"
	accountDomain "github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/authentication"
	"github.com/su3i/wimp/internal/infrastructure/cache"
)

// IssueTokens builds an access + refresh token pair for an already-resolved,
// already-authenticated account. Callers (password login, MFA-verified login) are
// expected to have fetched and validated the account themselves - this is the single
// shared call site so that work isn't repeated here.
func IssueTokens(acc *accountDomain.Account, commonCfg *config.CommonConfig, databaseCfg *config.DatabaseConfig) (*authentication.LoginDTO, error) {
	internalRoles := make([]string, 0, len(acc.InternalRoles))

	for _, v := range acc.InternalRoles {
		internalRoles = append(internalRoles, v)
	}

	accessToken, err := authentication.GenerateJWT(authentication.JWTParams{
		Subject:   acc.ID,
		Username:  acc.Username,
		Roles:     internalRoles,
		TTL:       time.Hour,
		SecretKey: []byte(commonCfg.JWTSecret),
	})

	if err != nil {
		return nil, err
	}

	refreshToken, refreshTokenHash, err := authentication.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := cache.GetCache().Set(fmt.Sprintf("refresh-token-%s", refreshTokenHash), acc.Username, 7*24*time.Hour); err != nil {
		return nil, errors.New("Failed to store refresh token")
	}

	return &authentication.LoginDTO{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func Refresh(rawRefresh string, commonCfg *config.CommonConfig, databaseCfg *config.DatabaseConfig) (*authentication.LoginDTO, error) {
	// 1. Get refresh token from cache
	oldRefreshTokenKey := fmt.Sprintf("refresh-token-%s", authentication.HashRefreshToken(rawRefresh))
	username, err := cache.GetCache().Get(oldRefreshTokenKey)

	if err != nil {
		return nil, errors.New("Invalid refresh token")
	}

	// 2. Get account
	_account, err := account.RetrieveAccount(username, databaseCfg)

	if err != nil {
		return nil, errors.New("Invalid account")
	}

	internalRoles := make([]string, 0, len(_account.InternalRoles))

	for _, v := range _account.InternalRoles {
		internalRoles = append(internalRoles, v)
	}

	// 3. Issue new access token
	accessToken, _ := authentication.GenerateJWT(authentication.JWTParams{
		Subject:   _account.ID,
		Username:  _account.Username,
		Roles:     internalRoles,
		TTL:       time.Hour,
		SecretKey: []byte(commonCfg.JWTSecret),
	})

	// 4. Rotate refresh token
	newRaw, newHash, err := authentication.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	err = cache.GetCache().Delete(oldRefreshTokenKey)

	if err != nil {
		return nil, errors.New("Failed to revoke refresh token")
	}

	err = cache.GetCache().Set(fmt.Sprintf("refresh-token-%s", newHash), username, 7*24*time.Hour)

	if err != nil {
		return nil, errors.New("Failed to rotate refresh token")
	}

	return &authentication.LoginDTO{
		AccessToken:  accessToken,
		RefreshToken: newRaw,
	}, nil
}
