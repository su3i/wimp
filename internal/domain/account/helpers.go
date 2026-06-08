package account

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/su3i/wimp/internal/domain/authorization"
	"golang.org/x/crypto/bcrypt"
)

func NewAccountRole(value string) (AccountRole, error) {
	switch AccountRole(value) {
	case SuperAdmin, Admin, Guest:
		return AccountRole(value), nil
	default:
		return "", errors.New("invalid account role")
	}
}

// BuildRoleKey builds an internal role key in the format:
// <domain>_<role>__<entityKey>
//
// Examples:
//  BuildRoleKey("org-1", AuthorizationDomainOrg, "superadmin")
//   -> "org_superadmin__org-1"
//
//  BuildRoleKey("project-9", AuthorizationDomainProject, "editor")
//   -> "project_editor__project-9"
func BuildRoleKey(entityKey string, domain authorization.AuthorizationDomain, role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	return fmt.Sprintf("%s_%s__%s", domain, role, entityKey)
}

func CheckPassword(password string) error {
	if password == "" {
		return errors.New("password must not be empty")
	}

	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

func EncryptPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func VerifyPassword(hashedPassword string, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(password),
	)
}

func GetSecurityLevel(account Account) SecurityLevel {
	score := 0

	if account.MFAEnabled {
		score += 3
	}

	if account.PasswordEnc != "" {
		score++
	}

	if account.Role == SuperAdmin || account.Role == Admin {
		score++
	}

	if len(account.InternalRoles) > 0 {
		score++
	}

	switch {
	case score >= 6:
		return SecurityExcellent
	case score >= 4:
		return SecurityStrong
	case score >= 2:
		return SecurityFair
	default:
		return SecurityWeak
	}
}
