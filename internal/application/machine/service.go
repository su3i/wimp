package machine

import (
	"errors"
	"fmt"
	"time"

	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/authentication"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func NewMachine(hostname string, projectKey string, cfg *config.DatabaseConfig) (*machine.Machine, error) {
	repo := database.NewMachineRepository(cfg)

	existing, err := repo.FindOneByHostname(hostname)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("machine with that hostname already exists")
	}

	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, errors.New("project not found")
	}

	token, _, err := authentication.GenerateRefreshToken()
	if err != nil {
		return nil, errors.New("failed to generate bootstrap token")
	}

	m := &machine.Machine{
		ProjectID:      proj.ID,
		Hostname:       hostname,
		Status:         machine.Pending,
		Token:          token,
		TokenExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return repo.Create(m)
}

func GetBootstrapToken(id uint, projectKey string, appHost string, cfg *config.DatabaseConfig) (string, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return "", errors.New("project not found")
	}

	repo := database.NewMachineRepository(cfg)

	m, err := repo.FindOneByID(id)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", errors.New("machine not found")
	}
	if m.ProjectID != proj.ID {
		return "", errors.New("machine not found")
	}
	if m.Token == "" {
		return "", errors.New("no bootstrap token available")
	}

	cmd := fmt.Sprintf(`iex (iwr "%s/bootstrap?token=%s").Content`, appHost, m.Token)
	return cmd, nil
}

func RetrieveMachines(projectKey string, cfg *config.DatabaseConfig) (*[]machine.Machine, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, errors.New("project not found")
	}

	repo := database.NewMachineRepository(cfg)
	return repo.FindByProjectID(proj.ID)
}
