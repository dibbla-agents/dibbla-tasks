//go:build !windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// executeInTerminal opens the command in a new native terminal window
// and returns immediately. The user can interact with the terminal
// (see output, Ctrl+C, etc.).
//
// On macOS: uses osascript to tell Terminal.app to open a new window.
// On Linux: tries common terminal emulators in order of popularity.
func (e *ShellExecutor) executeInTerminal(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	dir := opts.Dir
	if dir == "" {
		dir, _ = os.Getwd()
	}

	// Build an environment export prefix so the terminal session
	// inherits the enriched PATH and step-specific env vars.
	envPrefix := e.buildTerminalEnvPrefix(opts.Env)

	// The full command: cd to dir, export env, then run the user's command.
	fullCmd := fmt.Sprintf("cd %s && %s%s", shellQuoteTerminal(dir), envPrefix, cmd)

	if runtime.GOOS == "darwin" {
		return e.openMacOSTerminal(ctx, fullCmd)
	}
	return e.openLinuxTerminal(ctx, fullCmd)
}

// openMacOSTerminal opens a new Terminal.app window with the given command.
func (e *ShellExecutor) openMacOSTerminal(ctx context.Context, fullCmd string) (*ExecResult, error) {
	// Escape for AppleScript string embedding.
	escaped := strings.ReplaceAll(fullCmd, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)

	script := fmt.Sprintf(`
tell application "Terminal"
	do script "%s"
	activate
end tell`, escaped)

	c := exec.CommandContext(ctx, "osascript", "-e", script)
	if err := c.Run(); err != nil {
		return nil, fmt.Errorf("failed to open Terminal.app: %w", err)
	}

	return &ExecResult{ExitCode: 0}, nil
}

// openLinuxTerminal tries common Linux terminal emulators.
func (e *ShellExecutor) openLinuxTerminal(ctx context.Context, fullCmd string) (*ExecResult, error) {
	// List of common Linux terminal emulators, tried in order.
	terminals := []struct {
		bin  string
		args func(cmd string) []string
	}{
		{"gnome-terminal", func(cmd string) []string { return []string{"--", "bash", "-c", cmd + "; exec bash"} }},
		{"konsole", func(cmd string) []string { return []string{"-e", "bash", "-c", cmd + "; exec bash"} }},
		{"xfce4-terminal", func(cmd string) []string { return []string{"-e", "bash -c '" + cmd + "; exec bash'"} }},
		{"xterm", func(cmd string) []string { return []string{"-e", "bash", "-c", cmd + "; exec bash"} }},
	}

	for _, t := range terminals {
		if _, err := exec.LookPath(t.bin); err == nil {
			c := exec.CommandContext(ctx, t.bin, t.args(fullCmd)...)
			if err := c.Start(); err != nil {
				continue // try next terminal
			}
			return &ExecResult{ExitCode: 0}, nil
		}
	}

	return nil, fmt.Errorf("no supported terminal emulator found (tried gnome-terminal, konsole, xfce4-terminal, xterm)")
}

// buildTerminalEnvPrefix generates export statements for the terminal
// session so it inherits the enriched PATH and step-specific env vars.
func (e *ShellExecutor) buildTerminalEnvPrefix(stepEnv map[string]string) string {
	var sb strings.Builder

	// Export PATH from the enriched environment.
	env := e.buildEnvMap(stepEnv)
	if path, ok := env["PATH"]; ok {
		sb.WriteString(fmt.Sprintf("export PATH=%s && ", shellQuoteTerminal(path)))
	}

	// Export step-specific env vars.
	for k, v := range stepEnv {
		if k == "PATH" {
			continue
		}
		sb.WriteString(fmt.Sprintf("export %s=%s && ", k, shellQuoteTerminal(v)))
	}

	return sb.String()
}

// shellQuoteTerminal wraps a value in single quotes for terminal embedding.
func shellQuoteTerminal(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}

// BuildTerminalAppleScript constructs the osascript command for opening
// Terminal.app. Exported for testing.
func BuildTerminalAppleScript(fullCmd string) string {
	escaped := strings.ReplaceAll(fullCmd, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`tell application "Terminal"
	do script "%s"
	activate
end tell`, escaped)
}
