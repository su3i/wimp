package site

import (
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

func RetrieveByMachineID(machineID uint, cfg *config.DatabaseConfig) (*[]siteDomain.Site, error) {
	return database.NewSiteRepository(cfg).FindByMachineID(machineID)
}
