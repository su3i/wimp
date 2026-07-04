package site

import (
	"errors"

	"github.com/su3i/wimp/internal/cache"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/protocol"
	siteDomain "github.com/su3i/wimp/internal/domain/site"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

func UpsertFromDiscovery(machineID uint, sites []protocol.SiteInfo, cfg *config.DatabaseConfig) error {
	records := make([]siteDomain.Site, 0, len(sites))
	for _, s := range sites {
		bindings := make([]siteDomain.Binding, 0, len(s.Bindings))
		for _, b := range s.Bindings {
			bindings = append(bindings, siteDomain.Binding{
				Protocol: b.Protocol,
				IP:       b.IP,
				Port:     b.Port,
				Hostname: b.Hostname,
			})
		}
		records = append(records, siteDomain.Site{
			MachineID:    machineID,
			Name:         s.Name,
			State:        s.State,
			PhysicalPath: s.PhysicalPath,
			AppPoolName:  s.AppPoolName,
			Bindings:     bindings,
		})
	}
	return database.NewSiteRepository(cfg).SyncFromDiscovery(machineID, records)
}

func SyncHeartbeat(machineID uint, runningNames []string, cfg *config.DatabaseConfig) error {
	return database.NewSiteRepository(cfg).SyncStates(machineID, runningNames)
}

func RetrieveByMachineID(machineID uint, page, perPage int, state string, cfg *config.DatabaseConfig) (*[]siteDomain.Site, int64, error) {
	sites, total, err := database.NewSiteRepository(cfg).FindByMachineIDFiltered(machineID, page, perPage, state)
	if err != nil || sites == nil {
		return sites, total, err
	}
	siteIDs := make([]uint, len(*sites))
	for i, s := range *sites {
		siteIDs[i] = s.ID
	}
	overrides := cache.GetSitePendingStates(siteIDs)
	for i := range *sites {
		if s, ok := overrides[(*sites)[i].ID]; ok {
			(*sites)[i].State = s
		}
	}
	return sites, total, nil
}

func FindOneByID(id uint, cfg *config.DatabaseConfig) (*siteDomain.Site, error) {
	s, err := database.NewSiteRepository(cfg).FindOneByID(id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("site not found")
	}
	return s, nil
}
