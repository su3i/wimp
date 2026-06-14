package machine

type MachineStatus string

const (
	Pending  MachineStatus = "pending"
	Online   MachineStatus = "online"
	Offline  MachineStatus = "offline"
	Deleting MachineStatus = "deleting"
)
