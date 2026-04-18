//go:build !windows

package toolcheck

// RefreshSystemPath is a no-op on non-Windows platforms.
// On macOS and Linux, tools installed via brew (or apt, etc.) are placed
// in directories already present in the process PATH, so no refresh is needed.
func RefreshSystemPath() error {
	return nil
}
