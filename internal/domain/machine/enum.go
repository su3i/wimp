package machine

type MachineStatus string

const (
	Pending MachineStatus = "pending"
	Online  MachineStatus = "online"
	Offline MachineStatus = "offline"
	// Reassigned means this row's physical machine has since been bootstrapped into
	// another project (or re-bootstrapped into this one), and now reports there instead.
	// The row is kept rather than deleted so its history, app pools, and past alerts stay
	// readable, but it is terminal: the agent will never connect back on this token, and
	// the machine is dropped from Prometheus service discovery so the project it left
	// stops collecting live metrics for a host it no longer owns.
	Reassigned MachineStatus = "reassigned"
	Deleting   MachineStatus = "deleting"
)

// OsType is chosen by the operator when the host is added, since it decides which install
// command they get. Everything about what the host runs (IIS, SQL Server, a cluster) is
// discovered by the agent instead.
type OsType string

const (
	OsWindows OsType = "windows"
	OsLinux   OsType = "linux"
)

// Supported reports whether an agent exists for this OS yet. Linux is accepted as a value
// so the data model is ready for it, but no install path exists.
func (o OsType) Supported() bool {
	return o == OsWindows
}

// Terminal reports whether a status means the row will never come back online on its own.
// Used to keep stale rows out of scrape targets and out of anything that assumes a status
// is merely transient.
func (s MachineStatus) Terminal() bool {
	return s == Reassigned || s == Deleting
}
