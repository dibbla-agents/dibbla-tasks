//go:build windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// executeInTerminal opens the command in a new cmd.exe window on Windows.
func (e *ShellExecutor) executeInTerminal(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	dir := opts.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Use "start" to open a new console window with cmd /K so it stays open.
	// - "" is an explicit empty window title; without it, "start" treats the
	//   first quoted string (e.g. the path) as the title and breaks.
	// - /D sets the working directory via start's own flag, avoiding a
	//   "cd /d ... &&" chain whose && would be consumed by the outer shell.
	// SysProcAttr.CmdLine bypasses Go's argument escaping which would
	// mangle double quotes when passed to cmd.exe.
	rawCmdLine := fmt.Sprintf(`cmd /C start "" /D "%s" cmd /K %s`, dir, cmd)
	c := exec.CommandContext(ctx, "cmd")
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: rawCmdLine,
	}

	// Build environment
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	c.Env = env

	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("failed to open terminal window: %w", err)
	}

	return &ExecResult{ExitCode: 0}, nil
}
