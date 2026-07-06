package project

import (
	"errors"

	"github.com/su3i/wimp/internal/application/account"
	organizationService "github.com/su3i/wimp/internal/application/organization"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/project"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func NewProject(name string, key string, createdByUsername string, cfg *config.DatabaseConfig) (*project.Project, error) {
	_projectRepository := database.NewProjectRepository(cfg)

	_project, err := _projectRepository.FindOneByKey(key)

	if err != nil {
		return nil, err
	}

	if _project != nil {
		return nil, errors.New("Project already exists.")
	}

	org, err := organizationService.RetrieveOrganization("default", cfg)
	if err != nil || org == nil {
		return nil, errors.New("Organization not found")
	}

	createdByAccount, err := account.RetrieveAccount(createdByUsername, cfg)

	if err != nil {
		return nil, errors.New("Failed to get account")
	}

	createdBy := map[string]string{
		"Username": createdByUsername,
		"Name":     createdByAccount.Name,
	}

	_project = &project.Project{
		OrganizationID: org.ID,
		Name:           name,
		Key:            key,
		Status:         project.Active,
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

func DeleteProject(key string, cfg *config.DatabaseConfig) error {
	repo := database.NewProjectRepository(cfg)
	p, err := repo.FindOneByKey(key)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("project not found")
	}
	return repo.Delete(p.ID)
}

func UpdateProject(
    key string,
    name *string,
    newKey *string,
    cfg *config.DatabaseConfig,
) (*project.Project, error) {
    _projectRepository := database.NewProjectRepository(cfg)

    _project, err := _projectRepository.FindOneByKey(key)
    if err != nil {
        return nil, err
    }

    if _project == nil {
        return nil, errors.New("Project not found")
    }

    if name != nil {
        _project.Name = *name
    }
    if newKey != nil {
        _project.Key = *newKey
    }

    if err := _projectRepository.Update(_project); err != nil {
        return nil, err
    }

    return _project, nil
}