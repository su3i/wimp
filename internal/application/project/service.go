package project

import (
	"errors"

	"github.com/su3i/wimp/internal/application/account"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/project"
	"github.com/su3i/wimp/internal/infrastructure/database"
	organizationService "github.com/su3i/wimp/internal/application/organization"
)

func NewProject(name string, key string, businessDomain string, orgKey string, createdByEmail string, cfg *config.DatabaseConfig) (*project.Project, error) {
	_projectRepository := database.NewProjectRepository(cfg)

	_project, err := _projectRepository.FindOneByKey(key)

	if err != nil {
		return nil, err
	}

	if _project != nil {
		return nil, errors.New("Project already exists.")
	}

	org, err := organizationService.RetrieveOrganization(orgKey, cfg)
	if err != nil || org == nil {
		return nil, errors.New("Organization not found")
	}

	createdByAccount, err := account.RetrieveAccount(createdByEmail, cfg)

	if err != nil {
		return nil, errors.New("Failed to get account")
	}

	createdBy := map[string]string{
		"Email": createdByEmail,
		"Name":  createdByAccount.Name,
	}

	_project = &project.Project{
		OrganizationID: org.ID,
		Name:           name,
		Key:            key,
		Status:         project.Active,
		BusinessDomain: businessDomain,
		CreatedBy:      createdBy,
	}

	return _projectRepository.Create(_project)
}

func RetrieveProject(key string, cfg *config.DatabaseConfig) (*project.Project, error) {
	_projectRepository := database.NewProjectRepository(cfg)

	return _projectRepository.FindOneByKey(key)
}

func RetrieveProjects(cfg *config.DatabaseConfig) (*[]project.Project, error) {
	_projectRepository := database.NewProjectRepository(cfg)

	return _projectRepository.Find()
}

func UpdateProject(
    key string,
    name *string,
    newKey *string,
    businessDomain *string,
    cfg *config.DatabaseConfig,
) (*project.Project, error) {
    _projectRepository := database.NewProjectRepository(cfg)

    // Find existing project
    _project, err := _projectRepository.FindOneByKey(key)
    if err != nil {
        return nil, err
    }

    if _project == nil {
        return nil, errors.New("Project not found")
    }

    // Update fields only if provided
    if name != nil {
        _project.Name = *name
    }
	if newKey != nil {
        _project.Key = *newKey
    }
    if businessDomain != nil {
        _project.BusinessDomain = *businessDomain
    }

    // Save updated project
    if err := _projectRepository.Update(_project); err != nil {
        return nil, err
    }

    return _project, nil
}