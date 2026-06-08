package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/organization"
)

type organizationRepository struct {
	db *gorm.DB
}

func (r *organizationRepository) FindOne(key string) (*organization.Organization, error) {
	var _organization organization.Organization

	query := map[string]interface{}{
		"key": key,
	}

	if err := r.db.Unscoped().Where(query).First(&_organization).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &_organization, nil
}

func (r *organizationRepository) Create(payload *organization.Organization) (*organization.Organization, error) {
	_organization := organization.Organization{Name: payload.Name, Key: payload.Key, Scope: payload.Scope}

	err := r.db.Create(&_organization).Error

	if err != nil {
		return nil, errors.New("failed to create organization: " + err.Error())
	}

	return &_organization, nil
}

func (r *organizationRepository) Update(payload *organization.Organization) error {
	err := r.db.Updates(payload).Error

	if err != nil {
		return errors.New("failed to update organization: " + err.Error())
	}

	return nil
}

func NewOrganizationRepository(db *gorm.DB) organization.OrganizationRepository {
	return &organizationRepository{db: db}
}