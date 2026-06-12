package apppool

type AppPoolRepository interface {
	FindByMachineID(machineID uint) (*[]AppPool, error)
	FindOneByID(id uint) (*AppPool, error)
	SyncFromDiscovery(machineID uint, pools []AppPool) error
	// SyncStates updates pool states and returns (stoppedNames, startedNames, error).
	// stoppedNames: pools that transitioned running→stopped this heartbeat.
	// startedNames: pools that transitioned stopped→running this heartbeat.
	SyncStates(machineID uint, runningNames []string) (stoppedNames []string, startedNames []string, err error)
}
