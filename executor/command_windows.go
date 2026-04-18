//go:build windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ShellExecutor runs commands using the system shell.
// On Windows, it uses cmd /C.
type ShellExecutor struct {
	procs *ProcTracker // tracks background process PIDs for cleanup on shutdown
}

// NewShellExecutor creates a new platform-appropriate ShellExecutor.
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{
		procs: NewProcTracker(),
	}
}

// Execute runs cmd via cmd /C and streams output to the writers
// provided in opts. It uses SysProcAttr.CmdLine to pass the raw
// command string to CreateProcess, bypassing Go's argument escaping
// which would mangle double quotes (e.g. for /f "tokens=...").
func (e *ShellExecutor) Execute(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	// Delegate to background or terminal mode if requested.
	switch opts.Mode {
	case ExecModeBackground:
		return e.executeBackground(ctx, cmd, opts)
	case ExecModeTerminal:
		return e.executeInTerminal(ctx, cmd, opts)
	}

	c := exec.CommandContext(ctx, "cmd")
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd /C ` + cmd,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW — suppress console popup from GUI app
	}

	// Set working directory
	if opts.Dir != "" {
		c.Dir = opts.Dir
	}

	// Build environment: inherit current env + merge opts.Env
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	c.Env = env

	// Wire output streams
	if opts.Stdout != nil {
		c.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		c.Stderr = opts.Stderr
	}

	err := c.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	return &ExecResult{ExitCode: exitCode}, nil
}

// executeBackground starts a command in the background without waiting for
// it to exit. It waits briefly to detect immediate crashes.
func (e *ShellExecutor) executeBackground(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	c := exec.CommandContext(ctx, "cmd")
	c.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       `cmd /C ` + cmd,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW — suppress console popup from GUI app
	}

	if opts.Dir != "" {
		c.Dir = opts.Dir
	}

	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	c.Env = env

	if opts.Stdout != nil {
		c.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		c.Stderr = opts.Stderr
	}

	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start background command: %w", err)
	}

	pid := c.Process.Pid
	e.procs.Add(TrackedProc{
		PID:        pid,
		StepName:   opts.StepName,
		ProjectDir: opts.ProjectDir,
		Port:       opts.Port,
		StartedAt:  time.Now(),
	})

	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	select {
	case err := <-done:
		e.procs.Remove(pid)
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return nil, fmt.Errorf("background command failed: %w", err)
			}
		}
		return &ExecResult{ExitCode: exitCode}, nil
	case <-time.After(2 * time.Second):
		// Process still running — clean up tracker when it eventually exits.
		go func() {
			<-done
			e.procs.Remove(pid)
		}()
		return &ExecResult{ExitCode: 0}, nil
	}
}

// Ensure ShellExecutor implements Executor.
var _ Executor = (*ShellExecutor)(nil)
