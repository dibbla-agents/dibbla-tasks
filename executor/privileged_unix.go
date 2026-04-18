//go:build !windows

package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// executePrivileged runs a command with elevated privileges on macOS/Linux.
// On macOS it uses `osascript` with `do shell script ... with administrator
// privileges`, which shows the native system authentication dialog.
//
// Output streaming is achieved by redirecting stdout/stderr to a temp file
// inside the script and tailing that file in a background goroutine.
func (e *ShellExecutor) executePrivileged(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	// Build environment exports for the script so that the privileged
	// command inherits the enriched PATH and step-specific env vars.
	envExports := e.buildEnvExports(opts.Env)

	// Create temp files for the script and its output.
	scriptFile, err := os.CreateTemp("", "dibbla-priv-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}
	defer os.Remove(scriptFile.Name())

	outputFile, err := os.CreateTemp("", "dibbla-priv-out-*.log")
	if err != nil {
		scriptFile.Close()
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close() // we'll open it for reading separately
	defer os.Remove(outputPath)

	// Write the script. It exports env vars, runs the command, and
	// redirects all output to the temp file. The exit code is written
	// to a separate marker file so we can retrieve it after osascript.
	exitCodePath := outputPath + ".exit"
	defer os.Remove(exitCodePath)

	scriptContent := fmt.Sprintf("#!/bin/bash\n%s\n(%s) > %q 2>&1\nEC=$?\necho $EC > %q\nexit $EC\n",
		envExports,
		cmd,
		outputPath,
		exitCodePath,
	)

	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		scriptFile.Close()
		return nil, fmt.Errorf("failed to write temp script: %w", err)
	}
	scriptFile.Close()

	// Make the script executable.
	if err := os.Chmod(scriptFile.Name(), 0700); err != nil {
		return nil, fmt.Errorf("failed to chmod temp script: %w", err)
	}

	// Build the osascript command.
	osaScript := BuildPrivilegedOsascript(scriptFile.Name())
	c := exec.CommandContext(ctx, "osascript", "-e", osaScript)

	// Start tailing the output file in the background.
	tailDone := make(chan struct{})
	go func() {
		defer close(tailDone)
		tailOutputFile(ctx, outputPath, opts.Stdout, opts.Stderr)
	}()

	// Run osascript (blocks until complete or user cancels).
	osaErr := c.Run()

	// Give the tail goroutine a moment to flush remaining output.
	time.Sleep(200 * time.Millisecond)

	// Read the exit code written by the script.
	exitCode := readExitCode(exitCodePath)

	// Wait for tail goroutine to finish.
	select {
	case <-tailDone:
	case <-time.After(2 * time.Second):
		// Don't hang forever
	}

	// Determine the result.
	if osaErr != nil {
		if exitErr, ok := osaErr.(*exec.ExitError); ok {
			// osascript exit code 1 with no exit code file likely means
			// the user cancelled the auth dialog.
			if exitCode == -1 {
				exitCode = exitErr.ExitCode()
			}
			return &ExecResult{ExitCode: exitCode}, nil
		}
		return nil, fmt.Errorf("privileged execution failed: %w", osaErr)
	}

	if exitCode == -1 {
		exitCode = 0 // osascript succeeded, script didn't write exit code
	}

	return &ExecResult{ExitCode: exitCode}, nil
}

// BuildPrivilegedOsascript constructs the osascript AppleScript string
// that runs a script with administrator privileges.
// Exported for testing.
func BuildPrivilegedOsascript(scriptPath string) string {
	escaped := strings.ReplaceAll(scriptPath, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`do shell script "bash '%s'" with administrator privileges`, escaped)
}

// buildEnvExports generates shell export statements for the merged
// environment so the privileged script runs with the correct env.
func (e *ShellExecutor) buildEnvExports(stepEnv map[string]string) string {
	var sb strings.Builder

	// Only export PATH and step-specific vars to keep the script small.
	// The privileged script runs as root, so we mainly need PATH.
	env := e.buildEnvMap(stepEnv)

	// Always export PATH if we have an enriched one.
	if path, ok := env["PATH"]; ok {
		sb.WriteString(fmt.Sprintf("export PATH=%s\n", shellQuote(path)))
	}

	// Export step-specific env vars.
	for k, v := range stepEnv {
		if k == "PATH" {
			continue // already handled
		}
		sb.WriteString(fmt.Sprintf("export %s=%s\n", k, shellQuote(v)))
	}

	return sb.String()
}

// buildEnvMap returns the merged environment as a map (same priority as buildEnv).
func (e *ShellExecutor) buildEnvMap(stepEnv map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range e.loginEnv {
		merged[k] = v
	}
	for _, entry := range os.Environ() {
		key, value := splitEnvEntry(entry)
		if key != "" {
			merged[key] = value
		}
	}
	for k, v := range stepEnv {
		merged[k] = v
	}
	return merged
}

// shellQuote wraps a value in single quotes for safe shell embedding.
func shellQuote(s string) string {
	// Replace single quotes inside the value with '\''
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}

// tailOutputFile reads from the output file and streams lines to the
// provided writers. It polls until the context is cancelled.
func tailOutputFile(ctx context.Context, path string, stdout, stderr io.Writer) {
	// Wait briefly for the file to be created/written.
	time.Sleep(100 * time.Millisecond)

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			if stdout != nil {
				fmt.Fprintln(stdout, line)
			}
		}

		// Check if we should stop.
		select {
		case <-ctx.Done():
			// Final flush
			for scanner.Scan() {
				line := scanner.Text()
				if stdout != nil {
					fmt.Fprintln(stdout, line)
				}
			}
			return
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// readExitCode reads the exit code from the marker file.
// Returns -1 if the file doesn't exist or can't be read.
func readExitCode(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(data))
	var code int
	if _, err := fmt.Sscanf(s, "%d", &code); err != nil {
		return -1
	}
	return code
}
