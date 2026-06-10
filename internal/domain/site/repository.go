package site

type SiteRepository interface {
	FindByMachineID(machineID uint) (*[]Site, error)
	FindOneByID(id uint) (*Site, error)
	ReplaceAll(machineID uint, sites []Site) error
	SyncStates(machineID uint, runningNames []string) error
}
