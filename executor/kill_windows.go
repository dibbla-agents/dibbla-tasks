//go:build windows

package executor

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const killGracePeriod = 3 * time.Second

// KillTracked terminates all tracked background processes and blocks
// until they exit.
func (e *ShellExecutor) KillTracked() {
	pids := e.procs.PIDs()
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		cmd := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid))
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
		_ = cmd.Run()
	}

	e.procs.WaitForPIDs(pids, killGracePeriod)
}

// KillByProject terminates tracked background processes belonging to
// the given project directory and blocks until they exit.
func (e *ShellExecutor) KillByProject(projectDir string) {
	pids := e.procs.PIDsByProject(projectDir)
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		cmd := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid))
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000,
		}
		_ = cmd.Run()
	}

	e.procs.WaitForPIDs(pids, killGracePeriod)
}

// TrackedProcesses returns all currently tracked background processes.
func (e *ShellExecutor) TrackedProcesses() []TrackedProc {
	return e.procs.All()
}
