//go:build !windows

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
// On Unix-like systems (macOS, Linux), it uses /bin/sh -c.
// It caches the user's login shell environment at creation time so
// that commands spawned from a packaged GUI app have the full PATH.
type ShellExecutor struct {
	loginEnv map[string]string // cached login shell environment (may be nil)
	procs    *ProcTracker      // tracks background process PIDs for cleanup on shutdown
}

// NewShellExecutor creates a new platform-appropriate ShellExecutor.
// It probes the user's login shell once to capture the full environment
// (PATH, GOPATH, NVM_DIR, etc.), which is then injected into every
// spawned command. This is necessary because macOS GUI apps launched
// from Finder/Spotlight receive only a minimal environment from launchd.
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{
		loginEnv: resolveLoginEnv(),
		procs:    NewProcTracker(),
	}
}

// Execute runs cmd via /bin/sh -c and streams output to the writers
// provided in opts.
//
// Environment priority (last wins):
//  1. Login shell env (enriched PATH from user's shell profile)
//  2. Current process env (os.Environ)
//  3. Step-specific env (opts.Env) — highest priority
func (e *ShellExecutor) Execute(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	// Delegate to privileged execution if requested.
	if opts.Privileged {
		return e.executePrivileged(ctx, cmd, opts)
	}

	// Delegate to background or terminal mode if requested.
	switch opts.Mode {
	case ExecModeBackground:
		return e.executeBackground(ctx, cmd, opts)
	case ExecModeTerminal:
		return e.executeInTerminal(ctx, cmd, opts)
	}

	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmd)

	// Set working directory
	if opts.Dir != "" {
		c.Dir = opts.Dir
	}

	// Build environment with correct priority layering using a
	// deduplication map. Last writer wins for duplicate keys.
	c.Env = e.buildEnv(opts.Env)

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
// it to exit. It waits briefly (2 seconds) to detect immediate crashes.
// If the process is still running after the grace period, the step succeeds.
func (e *ShellExecutor) executeBackground(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	c := exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if opts.Dir != "" {
		c.Dir = opts.Dir
	}
	c.Env = e.buildEnv(opts.Env)

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

	// Wait briefly to detect immediate crashes (e.g. bad command, missing binary).
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	select {
	case err := <-done:
		e.procs.Remove(pid)
		// Process exited within the grace period — likely a crash.
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

// buildEnv constructs the environment variable slice for a child process.
// Priority (last wins): loginEnv -> os.Environ() -> stepEnv
func (e *ShellExecutor) buildEnv(stepEnv map[string]string) []string {
	merged := make(map[string]string)

	// Layer 1: Login shell env (enriched PATH, GOPATH, etc.)
	for k, v := range e.loginEnv {
		merged[k] = v
	}

	// Layer 2: Current process env (may override login env entries)
	for _, entry := range os.Environ() {
		key, value := splitEnvEntry(entry)
		if key != "" {
			merged[key] = value
		}
	}

	// Layer 3: Step-specific env (highest priority)
	for k, v := range stepEnv {
		merged[k] = v
	}

	// Convert map to slice
	env := make([]string, 0, len(merged))
	for k, v := range merged {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// splitEnvEntry splits a "KEY=VALUE" string into key and value.
// Returns empty key for malformed entries.
func splitEnvEntry(entry string) (string, string) {
	for i := 0; i < len(entry); i++ {
		if entry[i] == '=' {
			return entry[:i], entry[i+1:]
		}
	}
	return "", ""
}

// Ensure ShellExecutor implements Executor.
var _ Executor = (*ShellExecutor)(nil)
