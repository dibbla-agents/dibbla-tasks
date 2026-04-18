// Package portalloc provides dynamic TCP port allocation for the step runner.
// It finds free ports at runtime so that templates never need to hardcode ports
// or kill processes on ports they don't own.
package portalloc

import (
	"fmt"
	"net"
	"sort"
)

// PortDef declares a named port with a preferred value and scan range.
type PortDef struct {
	Preferred   int    `yaml:"preferred"              json:"preferred"`
	Description string `yaml:"description,omitempty"  json:"description,omitempty"`
	Range       int    `yaml:"range,omitempty"         json:"range,omitempty"` // default 100
}

func (d PortDef) effectiveRange() int {
	if d.Range > 0 {
		return d.Range
	}
	return 100
}

// Allocate finds a free TCP port, starting at def.Preferred and scanning
// up to def.Preferred + range. If the entire range is exhausted it falls
// back to an OS-assigned ephemeral port. The excluded set allows callers
// to prevent collisions between multiple allocations.
func Allocate(def PortDef, excluded map[int]bool) (int, error) {
	r := def.effectiveRange()

	for offset := 0; offset <= r; offset++ {
		port := def.Preferred + offset
		if port > 65535 {
			break
		}
		if excluded[port] {
			continue
		}
		if isPortFree(port) {
			return port, nil
		}
	}

	// Fallback: ask the OS for any free port.
	port, err := osAssignedPort()
	if err != nil {
		return 0, fmt.Errorf("portalloc: no free port in range %d–%d and OS fallback failed: %w",
			def.Preferred, def.Preferred+r, err)
	}
	return port, nil
}

// AllocateAll resolves all named ports, ensuring no two names receive
// the same port. Names are processed in sorted order for determinism.
func AllocateAll(defs map[string]PortDef) (map[string]int, error) {
	// Sort names for deterministic allocation order.
	names := make([]string, 0, len(defs))
	for name := range defs {
		names = append(names, name)
	}
	sort.Strings(names)

	allocated := make(map[string]int, len(defs))
	excluded := make(map[int]bool)

	for _, name := range names {
		port, err := Allocate(defs[name], excluded)
		if err != nil {
			return nil, fmt.Errorf("portalloc: failed to allocate port %q: %w", name, err)
		}
		allocated[name] = port
		excluded[port] = true
	}

	return allocated, nil
}

// isPortFree attempts to bind the given TCP port. Returns true if the
// port is available. Works cross-platform (macOS, Linux, Windows).
func isPortFree(port int) bool {
	ln, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// osAssignedPort asks the OS to assign an ephemeral port.
func osAssignedPort() (int, error) {
	ln, err := net.Listen("tcp4", ":0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}
