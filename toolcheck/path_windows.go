//go:build windows

package toolcheck

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RefreshSystemPath merges the current Machine and User PATH values from
// the Windows Registry into the running process's PATH. Any directory
// present in the registry but missing from the current process PATH is
// appended. Existing entries are preserved so we never lose directories
// that were present at startup.
//
// Registry PATH values are stored as REG_EXPAND_SZ and may contain
// unexpanded variables like %LOCALAPPDATA%. These are expanded before
// comparison and merging.
func RefreshSystemPath() error {
	sysPath, err := readRegistryPath(
		registry.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`,
	)
	if err != nil {
		return fmt.Errorf("read system PATH: %w", err)
	}

	userPath, _ := readRegistryPath(
		registry.CURRENT_USER,
		`Environment`,
	)

	// Collect all expanded registry directories.
	var registryDirs []string
	for _, raw := range []string{sysPath, userPath} {
		if raw == "" {
			continue
		}
		for _, entry := range strings.Split(raw, ";") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				registryDirs = append(registryDirs, entry)
			}
		}
	}

	if len(registryDirs) == 0 {
		return nil
	}

	// Build a set of existing process PATH entries (case-insensitive,
	// since Windows paths are case-insensitive).
	currentPath := os.Getenv("PATH")
	existing := make(map[string]bool)
	for _, entry := range strings.Split(currentPath, ";") {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			existing[strings.ToLower(entry)] = true
		}
	}

	// Append any registry entries not already present.
	var added []string
	for _, dir := range registryDirs {
		key := strings.ToLower(dir)
		if !existing[key] {
			existing[key] = true
			added = append(added, dir)
		}
	}

	if len(added) == 0 {
		return nil
	}

	newPath := currentPath + ";" + strings.Join(added, ";")
	return os.Setenv("PATH", newPath)
}

// readRegistryPath opens the given registry key, reads its "Path" value,
// and expands any embedded environment variables (e.g. %LOCALAPPDATA%).
func readRegistryPath(root registry.Key, subKey string) (string, error) {
	k, err := registry.OpenKey(root, subKey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, valType, err := k.GetStringValue("Path")
	if err != nil {
		return "", err
	}

	// REG_EXPAND_SZ values contain unexpanded variables like %SystemRoot%.
	if valType == registry.EXPAND_SZ {
		expanded, err := registry.ExpandString(val)
		if err != nil {
			return "", fmt.Errorf("expand PATH value: %w", err)
		}
		return expanded, nil
	}

	return val, nil
}
