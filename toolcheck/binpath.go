package toolcheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// dibblEnvFile is the path to the Dibbla environment file that stores
// PATH entries for tools installed to non-standard locations. This file
// is sourced by "Open With" terminal commands (e.g. Claude Code) so
// that externally launched processes inherit the enriched PATH.
const dibblaEnvFile = ".dibbla/env"

// ExpandBinPath expands ~ to the user's home directory in a bin_path string.
func ExpandBinPath(binPath string) (string, error) {
	if !strings.HasPrefix(binPath, "~") {
		return binPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand bin_path: %w", err)
	}
	return filepath.Join(home, binPath[1:]), nil
}

// PrependBinPath adds the given directory to the front of the process
// PATH environment variable if it is not already present.
func PrependBinPath(dir string) error {
	current := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(current) {
		if entry == dir {
			return nil // already present
		}
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+current)
}

// PersistBinPath appends a PATH export line to ~/.dibbla/env if the
// directory is not already recorded there. The file is created (along
// with the ~/.dibbla directory) if it does not exist.
//
// The generated lines use $HOME so the file is portable across sessions:
//
//	export PATH="$HOME/.local/go/bin:$PATH"
func PersistBinPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("persist bin_path: %w", err)
	}

	envPath := filepath.Join(home, dibblaEnvFile)

	// Build the export line using $HOME for portability.
	homePrefixedDir := dir
	if strings.HasPrefix(dir, home) {
		homePrefixedDir = "$HOME" + dir[len(home):]
	}
	exportLine := fmt.Sprintf("export PATH=\"%s:$PATH\"", homePrefixedDir)

	// Check if this line already exists.
	if fileContainsLine(envPath, exportLine) {
		return nil
	}

	// Ensure ~/.dibbla/ directory exists.
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return fmt.Errorf("create dibbla config dir: %w", err)
	}

	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open dibbla env file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, exportLine); err != nil {
		return fmt.Errorf("write to dibbla env file: %w", err)
	}

	return nil
}

// fileContainsLine checks whether the file at path contains the exact
// line (compared after trimming whitespace).
func fileContainsLine(path, line string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false // file doesn't exist or can't be read
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == strings.TrimSpace(line) {
			return true
		}
	}
	return false
}
