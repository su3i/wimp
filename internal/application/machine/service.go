package machine

import (
	"errors"
	"fmt"
	"time"

	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/authentication"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

type MachineWithPools struct {
	machine.Machine
	AppPools []apppool.AppPool `json:"app_pools"`
}

func NewMachine(projectKey string, cfg *config.DatabaseConfig) (*machine.Machine, error) {
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
		Status:         machine.Pending,
		Token:          token,
		TokenExpiresAt: time.Now().Add(24 * time.Hour),
	}

	repo := database.NewMachineRepository(cfg)
	return repo.Create(m)
}

// BelongsToProject verifies a machine exists and belongs to the given project. Use this
// for a plain ownership check - GetBootstrapToken also does this check internally but
// additionally builds install/uninstall PowerShell commands, which is wasted work (and
// a misleading call site) for callers that only need the existence check.
func BelongsToProject(id uint, projectKey string, cfg *config.DatabaseConfig) error {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return errors.New("project not found")
	}

	m, err := database.NewMachineRepository(cfg).FindOneByID(id)
	if err != nil {
		return err
	}
	if m == nil || m.ProjectID != proj.ID {
		return errors.New("machine not found")
	}
	return nil
}

func GetBootstrapToken(id uint, projectKey string, appUrl string, appEnv string, cfg *config.DatabaseConfig) (string, string, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return "", "", errors.New("project not found")
	}

	repo := database.NewMachineRepository(cfg)

	m, err := repo.FindOneByID(id)
	if err != nil {
		return "", "", err
	}
	if m == nil {
		return "", "", errors.New("machine not found")
	}
	if m.ProjectID != proj.ID {
		return "", "", errors.New("machine not found")
	}
	if m.Token == "" {
		return "", "", errors.New("no bootstrap token available")
	}

	iwrFlags := ""
	if appEnv == "development" {
		iwrFlags = " -Headers @{'ngrok-skip-browser-warning'='1'}"
	}

	downloadCmd := fmt.Sprintf(
		"iwr \"%s/bootstrap?token=%s\"%s -OutFile bootstrap.ps1",
		appUrl,
		m.Token,
		iwrFlags,
	)
	runCmd := "powershell -ExecutionPolicy Bypass -File bootstrap.ps1"
	return downloadCmd, runCmd, nil
}

func GetUninstallCommand(id uint, projectKey string, appUrl string, appEnv string, cfg *config.DatabaseConfig) (string, error) {
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

	iwrFlags := ""
	if appEnv == "development" {
		iwrFlags = " -Headers @{'ngrok-skip-browser-warning'='1'}"
	}

	cmd := fmt.Sprintf(
		"iwr \"%s/bootstrap/uninstall\"%s -OutFile cleanup.ps1\npowershell -ExecutionPolicy Bypass -File cleanup.ps1",
		appUrl,
		iwrFlags,
	)
	return cmd, nil
}

func RetrieveMachines(projectKey string, page, perPage int, status string, cfg *config.DatabaseConfig) ([]MachineWithPools, int64, error) {
	proj, err := projectService.RetrieveProject(projectKey, cfg)
	if err != nil || proj == nil {
		return nil, 0, errors.New("project not found")
	}

	machines, total, err := database.NewMachineRepository(cfg).FindByProjectIDFiltered(proj.ID, page, perPage, status)
	if err != nil {
		return nil, 0, err
	}

	appPoolRepo := database.NewAppPoolRepository(cfg)
	result := make([]MachineWithPools, len(*machines))
	for i, m := range *machines {
		pools, err := appPoolRepo.FindByMachineID(m.ID)
		if err != nil {
			return nil, 0, err
		}
		if pools == nil {
			pools = &[]apppool.AppPool{}
		}
		result[i] = MachineWithPools{Machine: m, AppPools: *pools}
	}

	return result, total, nil
}

// RequestDeletion marks the machine as deleting and returns the uninstall command.
// The machine is hard-deleted in ws.go when it disconnects while in this state.
func RequestDeletion(id uint, projectKey string, appUrl string, appEnv string, cfg *config.DatabaseConfig) (string, error) {
	downloadCmd, err := GetUninstallCommand(id, projectKey, appUrl, appEnv, cfg)
	if err != nil {
		return "", err
	}

	repo := database.NewMachineRepository(cfg)
	m, err := repo.FindOneByID(id)
	if err != nil || m == nil {
		return "", errors.New("machine not found")
	}

	m.Status = machine.Deleting
	if err := repo.Update(m); err != nil {
		return "", err
	}

	return downloadCmd, nil
}

// HardDelete permanently removes a machine and all its app pools.
func HardDelete(id uint, cfg *config.DatabaseConfig) error {
	if err := database.NewAppPoolRepository(cfg).DeleteByMachineID(id); err != nil {
		return err
	}
	return database.NewMachineRepository(cfg).Delete(id)
}
