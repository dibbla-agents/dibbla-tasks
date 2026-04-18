//go:build windows

package executor

// resolveLoginEnv is a no-op on Windows. Windows does not have the
// GUI-vs-terminal environment mismatch that macOS/Linux have with
// launchd. The process environment is always complete.
func resolveLoginEnv() map[string]string {
	return nil
}

// LoginEnvPATH is a no-op on Windows — the process PATH is always
// correct regardless of how the app is launched.
func LoginEnvPATH() string {
	return ""
}
