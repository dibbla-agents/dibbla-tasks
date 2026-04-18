package steprunner

import (
	"io"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
)

// Option configures a Run or RunReader invocation.
type Option func(*runConfig)

// runConfig holds all configuration for a pipeline run.
type runConfig struct {
	formatter    model.Formatter
	workDir      string
	env          map[string]string
	stdout       io.Writer
	stderr       io.Writer
	exec         executor.Executor
	killTracked  bool
}

// WithFormatter sets the output formatter used during execution.
// If not set, a GHActionsFormatter writing to the configured stdout is used.
func WithFormatter(f model.Formatter) Option {
	return func(c *runConfig) {
		c.formatter = f
	}
}

// WithWorkDir sets the base working directory for command execution.
func WithWorkDir(dir string) Option {
	return func(c *runConfig) {
		c.workDir = dir
	}
}

// WithEnv sets additional environment variables available to all steps.
func WithEnv(env map[string]string) Option {
	return func(c *runConfig) {
		c.env = env
	}
}

// WithOutputStream sets the io.Writer where stdout output is written.
// Defaults to os.Stdout.
func WithOutputStream(w io.Writer) Option {
	return func(c *runConfig) {
		c.stdout = w
	}
}

// WithErrStream sets the io.Writer where stderr output is written.
// Defaults to os.Stderr.
func WithErrStream(w io.Writer) Option {
	return func(c *runConfig) {
		c.stderr = w
	}
}

// WithExecutor sets the command executor. If not set, a new
// ShellExecutor is created per run. Sharing an executor across runs
// allows the caller to track and kill background processes on shutdown.
func WithExecutor(e executor.Executor) Option {
	return func(c *runConfig) {
		c.exec = e
	}
}

// WithKillTracked enables killing all tracked background processes from
// previous runs before execution begins. Off by default so that unrelated
// runs (e.g. "Open With") don't accidentally kill servers started by an
// earlier install.
func WithKillTracked() Option {
	return func(c *runConfig) {
		c.killTracked = true
	}
}
