//go:build windows

package executor

import (
	"context"
	"fmt"
)

// executePrivileged is not supported on Windows.
func (e *ShellExecutor) executePrivileged(ctx context.Context, cmd string, opts ExecOpts) (*ExecResult, error) {
	return nil, fmt.Errorf("privileged execution is not supported on Windows")
}

// BuildPrivilegedOsascript is a no-op stub on Windows.
func BuildPrivilegedOsascript(scriptPath string) string {
	return ""
}
