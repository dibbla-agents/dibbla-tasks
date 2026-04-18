// Package executor provides a cross-platform interface for running shell
// commands. Platform-specific implementations are selected at compile time
// via build tags.
package executor

import (
	"context"
	"io"
)

// ExecMode controls how a command is executed.
type ExecMode string

const (
	// ExecModeForeground (default) blocks until the command completes.
	ExecModeForeground ExecMode = ""

	// ExecModeBackground starts the command and returns immediately.
	// The step is marked successful if the process doesn't crash
	// within a brief grace period.
	ExecModeBackground ExecMode = "background"

	// ExecModeTerminal opens the command in a new native terminal
	// window and returns immediately.
	ExecModeTerminal ExecMode = "terminal"
)

// ExecOpts configures a single command execution.
type ExecOpts struct {
	// Env sets additional environment variables for the command.
	// These are merged with the current process environment.
	Env map[string]string

	// Dir sets the working directory. If empty, the current working
	// directory is used.
	Dir string

	// Stdout receives real-time standard output from the command.
	// If nil, output is discarded.
	Stdout io.Writer

	// Stderr receives real-time standard error from the command.
	// If nil, output is discarded.
	Stderr io.Writer

	// Privileged requests elevated privileges for the command.
	// On macOS this shows the native system authentication dialog
	// via osascript. If the user cancels, the command fails.
	Privileged bool

	// Mode controls the execution style. Default (empty) is foreground.
	// "background" starts the command without waiting for exit.
	// "terminal" opens the command in a new native terminal window.
	Mode ExecMode

	// StepName is the human-readable name of the step (for process tracking).
	StepName string

	// ProjectDir is the root project directory (for process tracking).
	ProjectDir string

	// Port is the primary port the process listens on, if known (for display).
	Port string
}

// ExecResult holds the outcome of a command execution.
type ExecResult struct {
	// ExitCode is the process exit code. 0 means success.
	ExitCode int
}

// Executor runs shell commands on the host OS.
type Executor interface {
	// Execute runs cmd as a shell command with the given options.
	// It blocks until the command completes or the context is cancelled.
	Execute(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error)
}
