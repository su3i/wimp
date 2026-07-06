package site

type SiteRepository interface {
	FindByMachineID(machineID uint) (*[]Site, error)
	FindByMachineIDFiltered(machineID uint, page, perPage int, state string) (*[]Site, int64, error)
	FindByMachineAndAppPool(machineID uint, appPoolName string) (*[]Site, error)
	FindOneByID(id uint) (*Site, error)
	SyncFromDiscovery(machineID uint, sites []Site) error
	// SyncStates updates site states and returns (stoppedNames, startedNames, error).
	SyncStates(machineID uint, runningNames []string) (stoppedNames []string, startedNames []string, err error)
}
