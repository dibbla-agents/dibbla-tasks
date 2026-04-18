// Package toolcheck provides cross-platform tool detection and installation.
// It uses exec.LookPath for detection and platform-specific install commands
// defined in the dibbla-task.yaml tools section.
package toolcheck

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
)

// CheckResult holds the outcome of a tool availability check.
type CheckResult struct {
	// Available is true if the tool's check command succeeded.
	Available bool

	// Path is the filesystem path to the tool binary, if found via LookPath.
	// May be empty if the tool was found only via check command.
	Path string

	// Output is the stdout from running the check command.
	Output string
}

// Checker detects tool availability and can install missing tools.
type Checker struct {
	platform Platform
	exec     executor.Executor
}

// NewChecker creates a new Checker using the given platform and executor.
func NewChecker(platform Platform, exec executor.Executor) *Checker {
	return &Checker{
		platform: platform,
		exec:     exec,
	}
}

// IsAvailable checks if a tool is available by:
// 1. Looking for the binary name in PATH (exec.LookPath)
// 2. Running the check command and verifying a zero exit code
//
// toolName is used for LookPath. checkCmd is the full command to run
// (e.g. "go version").
func (c *Checker) IsAvailable(ctx context.Context, toolName string, checkCmd string) (*CheckResult, error) {
	result := &CheckResult{}

	// Try LookPath first for a quick check
	path, err := exec.LookPath(toolName)
	if err == nil {
		result.Path = path
	}

	// Run the check command for a definitive answer
	if checkCmd != "" {
		var stdout safeBuffer
		execResult, err := c.exec.Execute(ctx, checkCmd, executor.ExecOpts{
			Stdout: &stdout,
		})
		if err != nil {
			return result, nil // command failed to run — tool not available
		}
		if execResult.ExitCode == 0 {
			result.Available = true
			result.Output = stdout.String()
		}
	} else if result.Path != "" {
		// No check command but found in PATH — consider it available
		result.Available = true
	}

	return result, nil
}

// Install runs the platform-specific install command for a tool.
// The toolDef provides the install commands and the privileged flag.
// Returns an error if no install command exists for the current platform
// or if the install command fails.
func (c *Checker) Install(ctx context.Context, toolName string, toolDef model.ToolDef, opts InstallOpts) error {
	platform := c.platform.OS()
	cmd, ok := toolDef.Install[platform]
	if !ok {
		return fmt.Errorf("no install command for tool %q on platform %q", toolName, platform)
	}

	execOpts := executor.ExecOpts{
		Privileged: toolDef.Privileged,
	}
	if opts.Stdout != nil {
		execOpts.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		execOpts.Stderr = opts.Stderr
	}

	result, err := c.exec.Execute(ctx, cmd, execOpts)
	if err != nil {
		return fmt.Errorf("install command for %q failed: %w", toolName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("install command for %q exited with code %d", toolName, result.ExitCode)
	}

	return nil
}

// InstallFallback runs the platform-specific fallback install command
// for a tool. Fallback commands are intended for unprivileged installs
// (e.g. curl to ~/.local) when the primary install fails.
// Returns an error if no fallback command exists for the current platform
// or if the fallback command fails.
func (c *Checker) InstallFallback(ctx context.Context, toolName string, toolDef model.ToolDef, opts InstallOpts) error {
	platform := c.platform.OS()
	cmd, ok := toolDef.InstallFallback[platform]
	if !ok {
		return fmt.Errorf("no fallback install command for tool %q on platform %q", toolName, platform)
	}

	execOpts := executor.ExecOpts{
		Privileged: false, // fallback installs never need elevation
	}
	if opts.Stdout != nil {
		execOpts.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		execOpts.Stderr = opts.Stderr
	}

	result, err := c.exec.Execute(ctx, cmd, execOpts)
	if err != nil {
		return fmt.Errorf("fallback install command for %q failed: %w", toolName, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("fallback install command for %q exited with code %d", toolName, result.ExitCode)
	}

	return nil
}

// InstallOpts configures tool installation.
type InstallOpts struct {
	// Stdout receives install command output.
	Stdout io.Writer
	// Stderr receives install command errors.
	Stderr io.Writer
}

// safeBuffer is a concurrency-safe bytes.Buffer used to capture command output.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
