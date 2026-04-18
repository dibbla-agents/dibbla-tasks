package runner

import (
	"fmt"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// FilterStepsForPlatform returns a new slice containing only the steps
// applicable to the given platform. Steps whose Platforms field is non-empty
// and does not include the target platform are removed. Dependency references
// to removed steps are pruned from the remaining steps so that the DAG
// stays valid.
func FilterStepsForPlatform(steps []model.Step, platform string) []model.Step {
	// Build the set of step IDs that will be kept.
	kept := make(map[string]bool, len(steps))
	for _, s := range steps {
		if len(s.Platforms) == 0 || containsPlatform(s.Platforms, platform) {
			kept[s.ID] = true
		}
	}

	var filtered []model.Step
	for _, s := range steps {
		if !kept[s.ID] {
			continue
		}
		// Prune depends_on references to removed steps.
		if len(s.DependsOn) > 0 {
			pruned := make([]string, 0, len(s.DependsOn))
			for _, dep := range s.DependsOn {
				if kept[dep] {
					pruned = append(pruned, dep)
				}
			}
			s.DependsOn = pruned
		}
		filtered = append(filtered, s)
	}
	return filtered
}

func containsPlatform(platforms []string, target string) bool {
	for _, p := range platforms {
		if p == target {
			return true
		}
	}
	return false
}

// TopologicalSort takes a list of steps and returns their IDs in a valid
// execution order that respects depends_on constraints.
func TopologicalSort(steps []model.Step) ([]string, error) {
	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps))

	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			dependents[dep] = append(dependents[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	var queue []string
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, dependent := range dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(steps) {
		return nil, fmt.Errorf("%w: processed %d of %d steps",
			model.ErrCyclicDependency, len(order), len(steps))
	}

	return order, nil
}
