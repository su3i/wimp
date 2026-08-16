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

// tokenNeverExpires is used as the agent token's expiry from the moment it's issued -
// there's no rotation path for it, so it just doesn't expire.
const tokenNeverExpires = 100 * 365 * 24 * time.Hour

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
		TokenExpiresAt: time.Now().Add(tokenNeverExpires),
	}

	repo := database.NewMachineRepository(cfg)
	return repo.Create(m)
}

// ReconcileReassignment retires any machine row that refers to the same physical box as
// the one that just registered. Re-running the bootstrap script on a host that is already
// in WIMP (typically to move it into another project) leaves the old row behind: the agent
// only ever holds one token, so it stops reporting against the old row entirely, and that
// row would otherwise sit at "offline" forever, indistinguishable from a crashed host, and
// keep serving Prometheus a scrape target for a box the project no longer owns.
//
// Called on every register. In the common case it finds nothing and costs one indexed query.
//
// Identity comes from the agent-reported machine UID. Rows that predate UID reporting are
// matched on hostname instead, but only when they are not currently connected - a host
// that is actively reporting is by definition not the box that just moved, and this is
// what stops two unrelated machines that merely share a hostname across projects from
// retiring each other.
func ReconcileReassignment(newMachine *machine.Machine, isOnline func(uint) bool, cfg *config.DatabaseConfig) []machine.Machine {
	if newMachine.MachineUID == "" && newMachine.Hostname == "" {
		return nil
	}

	repo := database.NewMachineRepository(cfg)

	candidates, err := repo.FindPredecessors(newMachine.ID, newMachine.MachineUID, newMachine.Hostname)
	if err != nil || candidates == nil {
		return nil
	}

	retired := make([]machine.Machine, 0, len(*candidates))
	for _, old := range *candidates {
		definite := newMachine.MachineUID != "" && old.MachineUID == newMachine.MachineUID
		if !definite && isOnline(old.ID) {
			continue
		}
		if err := repo.MarkReassigned(old.ID, newMachine.ID); err != nil {
			continue
		}
		retired = append(retired, old)
	}

	return retired
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
