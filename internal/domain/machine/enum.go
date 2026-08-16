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

// Terminal reports whether a status means the row will never come back online on its own.
// Used to keep stale rows out of scrape targets and out of anything that assumes a status
// is merely transient.
func (s MachineStatus) Terminal() bool {
	return s == Reassigned || s == Deleting
}
