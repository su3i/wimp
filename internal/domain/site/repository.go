package site

type SiteRepository interface {
	FindByMachineID(machineID uint) (*[]Site, error)
	FindByMachineAndAppPool(machineID uint, appPoolName string) (*[]Site, error)
	FindOneByID(id uint) (*Site, error)
	SyncFromDiscovery(machineID uint, sites []Site) error
	SyncStates(machineID uint, runningNames []string) error
}
