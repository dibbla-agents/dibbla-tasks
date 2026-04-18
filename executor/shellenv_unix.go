//go:build !windows

package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// resolveLoginEnv probes the user's login shell to capture the full
// environment (PATH, GOPATH, NVM_DIR, etc.). This is necessary because
// macOS GUI apps launched from Finder/Spotlight receive a minimal
// environment from launchd (PATH=/usr/bin:/bin:/usr/sbin:/sbin).
//
// The probe explicitly sources both profile AND rc files because:
//   - `zsh -l -c ...` is login but non-interactive, so it sources
//     ~/.zprofile but NOT ~/.zshrc
//   - Many users (and Homebrew's own setup instructions) place PATH
//     modifications in ~/.zshrc rather than ~/.zprofile
//   - The same applies to bash: ~/.bash_profile vs ~/.bashrc
//
// Returns nil on any failure — the caller should fall back to os.Environ().
func resolveLoginEnv() map[string]string {
	shell := detectShell()

	// Build a command that sources the rc file explicitly after the
	// login profile, with all output from the sourcing suppressed.
	// The env -0 at the end captures the complete environment.
	probeCmd := buildProbeCommand(shell)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", probeCmd)

	// Prevent the login shell from reading stdin (avoids hangs if
	// the shell config tries to read interactively).
	cmd.Stdin = nil

	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseNullTerminatedEnv(string(out))
}

// buildProbeCommand returns the shell command string that sources the
// appropriate rc file and then prints the environment. The rc file
// sourcing is wrapped with output suppression so only env -0 output
// reaches stdout.
func buildProbeCommand(shell string) string {
	// Determine which rc file to source based on shell type.
	// zsh -l already sources ~/.zprofile; bash -l already sources
	// ~/.bash_profile. We add the rc file on top.
	var rcFile string
	if strings.HasSuffix(shell, "/zsh") || strings.HasSuffix(shell, "/zsh5") {
		rcFile = "~/.zshrc"
	} else if strings.HasSuffix(shell, "/bash") {
		rcFile = "~/.bashrc"
	}

	if rcFile != "" {
		// Source the rc file with all its output suppressed (both stdout
		// and stderr), then run env -0 whose output goes to real stdout.
		return fmt.Sprintf("test -f %s && source %s >/dev/null 2>&1; env -0", rcFile, rcFile)
	}

	// For sh or unknown shells, just run env -0 (login profile is
	// already sourced via -l).
	return "env -0"
}

// detectShell returns the path to the user's preferred shell.
// It checks $SHELL first, then falls back to /bin/zsh (macOS default
// since Catalina), then /bin/sh as the ultimate fallback.
func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	// macOS default since Catalina
	if _, err := os.Stat("/bin/zsh"); err == nil {
		return "/bin/zsh"
	}

	return "/bin/sh"
}

// parseNullTerminatedEnv parses the output of `env -0` into a map.
// Each entry is terminated by a null byte and formatted as KEY=VALUE.
func parseNullTerminatedEnv(data string) map[string]string {
	if data == "" {
		return nil
	}

	entries := strings.Split(data, "\x00")
	env := make(map[string]string, len(entries))

	for _, entry := range entries {
		if entry == "" {
			continue
		}
		idx := strings.Index(entry, "=")
		if idx <= 0 {
			continue
		}
		key := entry[:idx]
		value := entry[idx+1:]
		env[key] = value
	}

	if len(env) == 0 {
		return nil
	}

	return env
}

// LoginEnvPATH resolves the user's login shell PATH and returns it.
// This is intended to be called early in main() to enrich the process
// PATH so that exec.LookPath works correctly in packaged GUI apps.
//
// If the login shell probe succeeds, it returns that PATH with any
// missing well-known developer tool directories appended.
// If the probe fails entirely, it returns the current PATH enriched
// with well-known directories as a best-effort fallback.
func LoginEnvPATH() string {
	env := resolveLoginEnv()

	var basePATH string
	if env != nil {
		basePATH = env["PATH"]
	}
	if basePATH == "" {
		basePATH = os.Getenv("PATH")
	}

	return ensureWellKnownPaths(basePATH)
}

// wellKnownPaths lists directories where common developer tools are
// installed on macOS. These are appended (not prepended) to the PATH
// so they act as a fallback, not an override.
var wellKnownPaths = []string{
	"/opt/homebrew/bin",      // Homebrew on Apple Silicon
	"/opt/homebrew/sbin",     // Homebrew sbin on Apple Silicon
	"/usr/local/bin",         // Homebrew on Intel, many other tools
	"/usr/local/sbin",        // Intel brew sbin
	"/usr/local/go/bin",      // Default Go install location
	"/Library/Apple/usr/bin", // Xcode command-line tools alternate
}

// ensureWellKnownPaths appends any missing well-known directories to
// the given PATH string. Directories that are already present are not
// duplicated.
func ensureWellKnownPaths(path string) string {
	existing := make(map[string]bool)
	for _, p := range strings.Split(path, ":") {
		existing[p] = true
	}

	var additions []string
	for _, p := range wellKnownPaths {
		if !existing[p] {
			// Only add if the directory actually exists on disk.
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				additions = append(additions, p)
			}
		}
	}

	if len(additions) == 0 {
		return path
	}

	return path + ":" + strings.Join(additions, ":")
}
