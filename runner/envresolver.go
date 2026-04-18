package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// ResolveEnv resolves pipeline-level environment variables declared in the
// task file's env section. It applies defaults, overlays user-provided
// values, and validates that all required variables have a value.
//
// Resolution order for each declared variable:
//  1. Default from the env declaration (if any).
//  2. Override from provided (user/frontend-supplied values via WithEnv).
//
// Variables present in provided but not declared in the env section are
// passed through unchanged — this allows runner-level env to coexist
// with declared pipeline env.
func ResolveEnv(declared map[string]model.EnvVarDef, provided map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)

	// 1. Apply defaults from declarations.
	for name, def := range declared {
		if def.Default != "" {
			resolved[name] = def.Default
		}
	}

	// 2. Overlay user-provided values (WithEnv / frontend input).
	for k, v := range provided {
		resolved[k] = v
	}

	// 3. Validate required vars.
	var missing []string
	for name, def := range declared {
		if def.Required {
			if _, ok := resolved[name]; !ok {
				missing = append(missing, name)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("%w: %s", model.ErrMissingRequiredEnv, strings.Join(missing, ", "))
	}

	return resolved, nil
}
