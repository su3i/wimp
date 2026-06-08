package authentication

import (
	"errors"
	"fmt"
	"time"

	"github.com/su3i/wimp/internal/application/account"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/authentication"
	"github.com/su3i/wimp/internal/infrastructure/cache"
)

func Login(email string, password string, commonCfg *config.CommonConfig, databaseCfg *config.DatabaseConfig) (*authentication.LoginDTO, error) {
	_account, err := account.RetrieveAccountWithPassword(email, password, databaseCfg)

	if err != nil || _account == nil {
		return nil, errors.New("Invalid email or password")
	}

	internalRoles := make([]string, 0, len(_account.InternalRoles))

	for _, v := range _account.InternalRoles {
		internalRoles = append(internalRoles, v)
	}

	accessToken, err := authentication.GenerateJWT(authentication.JWTParams{
		Subject:   _account.ID,
		Email:     _account.Email,
		Roles:	   internalRoles,
		TTL:       time.Hour,
		SecretKey: []byte(commonCfg.JWTSecret),
	})
	
	if err != nil {
		return nil, err
	}

	refreshToken, refreshTokenHash, err := authentication.GenerateRefreshToken()

	err = cache.GetCache().Set(fmt.Sprintf("refresh-token-%s", refreshTokenHash), email, 7*24*time.Hour)

	if err != nil {
		return nil, errors.New("Failed to rotate refresh token")
	}

	return &authentication.LoginDTO{
		AccessToken: accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func LoginWithoutPassword(email string, commonCfg *config.CommonConfig, databaseCfg *config.DatabaseConfig) (*authentication.LoginDTO, error) {
	_account, err := account.RetrieveAccount(email, databaseCfg)

	if err != nil || _account == nil {
		return nil, errors.New("Invalid email or password")
	}

	internalRoles := make([]string, 0, len(_account.InternalRoles))

	for _, v := range _account.InternalRoles {
		internalRoles = append(internalRoles, v)
	}

	accessToken, err := authentication.GenerateJWT(authentication.JWTParams{
		Subject:   _account.ID,
		Email:     _account.Email,
		Roles:	   internalRoles,
		TTL:       time.Hour,
		SecretKey: []byte(commonCfg.JWTSecret),
	})
	
	if err != nil {
		return nil, err
	}

	refreshToken, refreshTokenHash, err := authentication.GenerateRefreshToken()

	err = cache.GetCache().Set(fmt.Sprintf("refresh-token-%s", refreshTokenHash), email, 7*24*time.Hour)

	if err != nil {
		return nil, errors.New("Failed to rotate refresh token")
	}
	
	return &authentication.LoginDTO{
		AccessToken: accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func Refresh(rawRefresh string, commonCfg *config.CommonConfig, databaseCfg *config.DatabaseConfig) (*authentication.LoginDTO, error) {
	// 1. Get refresh token from repository
	oldRefreshTokenKey := fmt.Sprintf("refresh-token-%s", authentication.HashRefreshToken(rawRefresh))
	email, err := cache.GetCache().Get(oldRefreshTokenKey)

	if err != nil {
		return nil, errors.New("Invalid refresh token")
	}

	// 2. Get account
	_account, err := account.RetrieveAccount(email, databaseCfg)

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
		Email:     _account.Email,
		Roles:	   internalRoles,
		TTL:       time.Hour,
		SecretKey: []byte(commonCfg.JWTSecret),
	})

	// 4. Rotate refresh token
	newRaw, newHash, _ := authentication.GenerateRefreshToken()

	err = cache.GetCache().Delete(oldRefreshTokenKey)

	if err != nil {
		return nil, errors.New("Failed to revoke refresh token")
	}
	
	err = cache.GetCache().Set(fmt.Sprintf("refresh-token-%s", newHash), email, 7*24*time.Hour)

	if err != nil {
		return nil, errors.New("Failed to rotate refresh token")
	}

	return &authentication.LoginDTO{
		AccessToken:  accessToken,
		RefreshToken: newRaw,
	}, nil
}