package sqlinstance

type SqlInstanceRepository interface {
	FindByMachineID(machineID uint) (*[]SqlInstance, error)
	// Sync makes the machine's rows match instances exactly: updates known ones, creates new
	// ones, deletes any no longer reported.
	// Called on discovery only, which is the one authoritative listing.
	Sync(machineID uint, instances []SqlInstance) error
	// UpdateStates writes the runtime fields (state, agent state, owner node) of instances
	// that already exist, and never creates or deletes. Called on every heartbeat, where a
	// transiently failed read must not churn rows that other records point at.
	UpdateStates(machineID uint, instances []SqlInstance) error
	DeleteByMachineID(machineID uint) error
}
