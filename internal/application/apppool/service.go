package apppool

import (
	"errors"

	"github.com/su3i/wimp/internal/cache"
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
	return database.NewAppPoolRepository(cfg).SyncFromDiscovery(machineID, records)
}

func SyncHeartbeat(machineID uint, runningNames []string, cfg *config.DatabaseConfig) (stoppedNames []string, startedNames []string, err error) {
	return database.NewAppPoolRepository(cfg).SyncStates(machineID, runningNames)
}

func RetrieveByMachineID(machineID uint, page, perPage int, state string, cfg *config.DatabaseConfig) (*[]apppool.AppPool, int64, error) {
	pools, total, err := database.NewAppPoolRepository(cfg).FindByMachineIDFiltered(machineID, page, perPage, state)
	if err != nil || pools == nil {
		return pools, total, err
	}
	poolIDs := make([]uint, len(*pools))
	for i, p := range *pools {
		poolIDs[i] = p.ID
	}
	overrides := cache.GetPoolPendingStates(poolIDs)
	for i := range *pools {
		if s, ok := overrides[(*pools)[i].ID]; ok {
			(*pools)[i].State = s
		}
	}
	return pools, total, nil
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
