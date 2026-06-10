package hub

import "sync"

type CommandResult struct {
	Success bool
	Output  string
	Error   string
}

var cmdHub = &commandHub{
	pending: make(map[string]chan CommandResult),
}

type commandHub struct {
	mu      sync.Mutex
	pending map[string]chan CommandResult
}

// RegisterCommand creates a result channel for the given command ID.
// The caller must call DeregisterCommand if the wait times out.
func RegisterCommand(id string) chan CommandResult {
	ch := make(chan CommandResult, 1)
	cmdHub.mu.Lock()
	cmdHub.pending[id] = ch
	cmdHub.mu.Unlock()
	return ch
}

// ResolveCommand delivers a result to the waiting caller and removes the entry.
func ResolveCommand(id string, result CommandResult) {
	cmdHub.mu.Lock()
	ch, ok := cmdHub.pending[id]
	if ok {
		delete(cmdHub.pending, id)
	}
	cmdHub.mu.Unlock()
	if ok {
		ch <- result
	}
}

// DeregisterCommand removes a pending command entry without delivering a result.
func DeregisterCommand(id string) {
	cmdHub.mu.Lock()
	delete(cmdHub.pending, id)
	cmdHub.mu.Unlock()
}
