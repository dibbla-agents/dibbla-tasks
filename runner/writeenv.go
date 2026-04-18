package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// WriteEnvFile writes resolved environment variables to a .env file.
// Variables are sorted alphabetically. If a declaration with a description
// exists for a variable, it is written as a comment above the variable.
//
// Output format:
//
//	# Description for DATABASE_URL
//	DATABASE_URL=postgres://localhost:5432/mydb
//
//	# Description for LOG_LEVEL
//	LOG_LEVEL=info
func WriteEnvFile(dir, filename string, vars map[string]string, defs map[string]model.EnvVarDef) error {
	if filename == "" {
		filename = ".env"
	}

	path := filepath.Join(dir, filename)

	// Collect and sort variable names for deterministic output.
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteString("\n")
		}
		// Write description comment if available.
		if def, ok := defs[name]; ok && def.Description != "" {
			fmt.Fprintf(&b, "# %s\n", def.Description)
		}
		fmt.Fprintf(&b, "%s=%s\n", name, vars[name])
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}
