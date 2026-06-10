package apppool

import (
	"errors"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func UpsertFromDiscovery(machineID uint, pools []protocol.AppPoolInfo, cfg *config.DatabaseConfig) error {
	records := make([]apppool.AppPool, 0, len(pools))
	for _, p := range pools {
		records = append(records, apppool.AppPool{
			MachineID:      machineID,
			Name:           p.Name,
			State:          p.State,
			RuntimeVersion: p.RuntimeVersion,
			PipelineMode:   p.PipelineMode,
			StartMode:      p.StartMode,
			IdentityType:   p.IdentityType,
		})
	}
	return database.NewAppPoolRepository(cfg).ReplaceAll(machineID, records)
}

func SyncHeartbeat(machineID uint, runningNames []string, cfg *config.DatabaseConfig) error {
	return database.NewAppPoolRepository(cfg).SyncStates(machineID, runningNames)
}

func RetrieveByMachineID(machineID uint, cfg *config.DatabaseConfig) (*[]apppool.AppPool, error) {
	return database.NewAppPoolRepository(cfg).FindByMachineID(machineID)
}

func FindOneByID(id uint, cfg *config.DatabaseConfig) (*apppool.AppPool, error) {
	pool, err := database.NewAppPoolRepository(cfg).FindOneByID(id)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, errors.New("app pool not found")
	}
	return pool, nil
}
