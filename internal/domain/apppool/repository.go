package apppool

type AppPoolRepository interface {
	FindByMachineID(machineID uint) (*[]AppPool, error)
	FindOneByID(id uint) (*AppPool, error)
	SyncFromDiscovery(machineID uint, pools []AppPool) error
	SyncStates(machineID uint, runningNames []string) error
}
