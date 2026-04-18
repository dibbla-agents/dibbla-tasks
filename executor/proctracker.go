package executor

import (
	"sync"
	"time"
)

// TrackedProc holds metadata about a background process spawned by Dibbla.
type TrackedProc struct {
	PID        int       `json:"pid"`
	StepName   string    `json:"stepName"`
	ProjectDir string    `json:"projectDir"`
	Port       string    `json:"port,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
}

// ProcTracker is a concurrency-safe registry of background processes.
// It allows the application to track spawned processes and kill them on
// shutdown.
type ProcTracker struct {
	mu      sync.Mutex
	procs   map[int]TrackedProc
	changed chan struct{} // signaled on every Add/Remove
}

// NewProcTracker creates a new empty ProcTracker.
func NewProcTracker() *ProcTracker {
	return &ProcTracker{
		procs:   make(map[int]TrackedProc),
		changed: make(chan struct{}, 1),
	}
}

// notify sends a non-blocking signal on the changed channel.
func (t *ProcTracker) notify() {
	select {
	case t.changed <- struct{}{}:
	default:
	}
}

// Add registers a background process with metadata.
func (t *ProcTracker) Add(proc TrackedProc) {
	t.mu.Lock()
	t.procs[proc.PID] = proc
	t.mu.Unlock()
	t.notify()
}

// Remove unregisters a PID (e.g. when the process exits naturally).
func (t *ProcTracker) Remove(pid int) {
	t.mu.Lock()
	delete(t.procs, pid)
	t.mu.Unlock()
	t.notify()
}

// PIDs returns a snapshot of all currently tracked PIDs.
func (t *ProcTracker) PIDs() []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]int, 0, len(t.procs))
	for pid := range t.procs {
		out = append(out, pid)
	}
	return out
}

// All returns a snapshot of all currently tracked processes.
func (t *ProcTracker) All() []TrackedProc {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TrackedProc, 0, len(t.procs))
	for _, proc := range t.procs {
		out = append(out, proc)
	}
	return out
}

// WaitForPIDs blocks until none of the given PIDs are tracked, or until
// the timeout expires. Returns the PIDs that are still tracked (empty on success).
func (t *ProcTracker) WaitForPIDs(pids []int, timeout time.Duration) []int {
	deadline := time.After(timeout)
	for {
		// Check if all target PIDs are gone.
		t.mu.Lock()
		var remaining []int
		for _, pid := range pids {
			if _, ok := t.procs[pid]; ok {
				remaining = append(remaining, pid)
			}
		}
		t.mu.Unlock()

		if len(remaining) == 0 {
			return nil
		}

		// Wait for a change or timeout.
		select {
		case <-t.changed:
			continue
		case <-deadline:
			return remaining
		}
	}
}

// PIDsByProject returns PIDs belonging to the given project directory.
func (t *ProcTracker) PIDsByProject(projectDir string) []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []int
	for pid, proc := range t.procs {
		if proc.ProjectDir == projectDir {
			out = append(out, pid)
		}
	}
	return out
}
