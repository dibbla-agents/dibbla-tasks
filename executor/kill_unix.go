//go:build !windows

package executor

import (
	"syscall"
	"time"
)

const killGracePeriod = 3 * time.Second

// KillTracked terminates all tracked background processes and blocks
// until they exit. It sends SIGTERM first, waits up to 3 seconds, then
// escalates to SIGKILL for any that remain.
func (e *ShellExecutor) KillTracked() {
	pids := e.procs.PIDs()
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}

	remaining := e.procs.WaitForPIDs(pids, killGracePeriod)
	for _, pid := range remaining {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	if len(remaining) > 0 {
		e.procs.WaitForPIDs(remaining, 1*time.Second)
	}
}

// KillByProject terminates tracked background processes belonging to
// the given project directory and blocks until they exit.
func (e *ShellExecutor) KillByProject(projectDir string) {
	pids := e.procs.PIDsByProject(projectDir)
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}

	remaining := e.procs.WaitForPIDs(pids, killGracePeriod)
	for _, pid := range remaining {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	if len(remaining) > 0 {
		e.procs.WaitForPIDs(remaining, 1*time.Second)
	}
}

// TrackedProcesses returns all currently tracked background processes.
func (e *ShellExecutor) TrackedProcesses() []TrackedProc {
	return e.procs.All()
}
