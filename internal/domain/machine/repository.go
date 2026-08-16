package machine

type MachineRepository interface {
	FindAll() (*[]Machine, error)
	FindByProjectID(projectID uint) (*[]Machine, error)
	FindByProjectIDFiltered(projectID uint, page, perPage int, status string) (*[]Machine, int64, error)
	FindOneByID(id uint) (*Machine, error)
	FindOneByHostname(hostname string) (*Machine, error)
	FindOneByToken(token string) (*Machine, error)
	// FindPredecessors returns every other machine row that looks like the same physical
	// box as excludeID, matched on machine UID when one is known and falling back to
	// hostname for rows predating UID reporting. Terminal rows are excluded - a row
	// already marked reassigned or deleting has nothing left to reconcile.
	FindPredecessors(excludeID uint, machineUID, hostname string) (*[]Machine, error)
	// MarkReassigned is a scoped column update rather than a full-row save: the machine
	// being marked may still have a live WebSocket whose read loop holds an older copy of
	// the row, and a full save from either side would clobber the other's fields.
	MarkReassigned(id, supersededBy uint) error
	// SetIdentity updates only the fields an agent reports at registration time.
	SetIdentity(id uint, hostname string, ips []string, agentVersion, windowsVersion, machineUID string) error
	Create(payload *Machine) (*Machine, error)
	Update(payload *Machine) error
	Delete(id uint) error
}
