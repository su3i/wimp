package organization

import (
	"errors"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/organization"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func NewOrganization(name string, key string, scope string, cfg *config.DatabaseConfig) (*organization.Organization, error) {
	_organizationRepository := database.NewOrganizationRepository(cfg)

	_organization, err := _organizationRepository.FindOne(key)

	if err != nil {
		return nil, err
	}

	if _organization != nil {
		return nil, errors.New("Organization already exists.")
	}

	_organization = &organization.Organization{
		Name:  name,
		Key:   key,
		Scope: organization.OrgScope(scope),
	}

	return _organizationRepository.Create(_organization)
}

func RetrieveOrganization(key string, cfg *config.DatabaseConfig) (*organization.Organization, error) {
	_organizationRepository := database.NewOrganizationRepository(cfg)

	return _organizationRepository.FindOne(key)
}

func UpdateOrganization(name *string, key string, scope *string, cfg *config.DatabaseConfig) (*organization.Organization, error) {
	_organizationRepository := database.NewOrganizationRepository(cfg)

	_organization, err := _organizationRepository.FindOne(key)

	if err != nil || _organization == nil {
		return nil, errors.New("Failed to get organization.")
	}

	if name != nil {
		_organization.Name = *name
	}

	if scope != nil {
		_organization.Scope = organization.OrgScope(*scope)
	}

	// Save updated project
	if err := _organizationRepository.Update(_organization); err != nil {
		return nil, err
	}

	return _organization, nil
}