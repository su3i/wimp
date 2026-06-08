package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/domain/project"
)

type projectRepository struct {
	db *gorm.DB
}

func (r *projectRepository) Find() (*[]project.Project, error) {
	var _projects []project.Project

	if err := r.db.Unscoped().Find(&_projects).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &_projects, nil
}

func (r *projectRepository) FindOneByKey(key string) (*project.Project, error) {
	var _project project.Project

	query := map[string]interface{}{
		"key": key,
	}

	if err := r.db.Unscoped().Where(query).First(&_project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &_project, nil
}

func (r *projectRepository) Create(payload *project.Project) (*project.Project, error) {
	_project := project.Project{
		OrganizationID: payload.OrganizationID,
		Name:           payload.Name,
		Key:            payload.Key,
		Status:         payload.Status,
		BusinessDomain: payload.BusinessDomain,
		CreatedBy:      payload.CreatedBy,
	}

	err := r.db.Create(&_project).Error

	if err != nil {
		return nil, errors.New("failed to create project: " + err.Error())
	}

	return &_project, nil
}

func (r *projectRepository) Update(payload *project.Project) error {
	err := r.db.Updates(payload).Error

	if err != nil {
		return errors.New("failed to update project: " + err.Error())
	}

	return nil
}

func NewProjectRepository(db *gorm.DB) project.ProjectRepository {
	return &projectRepository{db: db}
}