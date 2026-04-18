package toolcheck

import "runtime"

// Platform abstracts OS detection so it can be overridden in tests.
type Platform interface {
	// OS returns the current operating system name: "darwin", "linux",
	// or "windows".
	OS() string
}

// RuntimePlatform uses runtime.GOOS for OS detection.
type RuntimePlatform struct{}

// OS returns runtime.GOOS.
func (RuntimePlatform) OS() string {
	return runtime.GOOS
}
