//go:build darwin

// Package xcodewatch detects in-progress Xcode Command Line Tools installations
// on macOS and emits formatter notices so the UI can display a banner.
package xcodewatch

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

const sentinelPath = "/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress"

// Watch starts a background goroutine that polls for the Xcode CLT sentinel
// file and emits notices through the formatter. Returns a cancel function that
// stops the watcher. The watcher also stops when ctx is cancelled.
//
// If CLT is already installed, returns a no-op cancel function immediately.
func Watch(ctx context.Context, formatter model.Formatter) context.CancelFunc {
	return watchWithConfig(ctx, formatter, watchConfig{
		sentinelPath:      sentinelPath,
		pollInterval:      2 * time.Second,
		heartbeatInterval: 30 * time.Second,
	})
}

// watchConfig holds tunables for the watcher, used by tests.
type watchConfig struct {
	sentinelPath      string
	pollInterval      time.Duration
	heartbeatInterval time.Duration
	skipInstallCheck  bool // when true, skip the isInstalled() gate
}

// watchWithConfig is the internal implementation with configurable parameters
// for testing.
func watchWithConfig(ctx context.Context, formatter model.Formatter, cfg watchConfig) context.CancelFunc {
	if !cfg.skipInstallCheck && isInstalled() {
		return func() {}
	}

	path := cfg.sentinelPath
	pollInterval := cfg.pollInterval
	heartbeatInterval := cfg.heartbeatInterval

	watchCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		detected := false
		var lastHeartbeat time.Time

		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				exists := fileExists(path)

				if exists && !detected {
					detected = true
					lastHeartbeat = time.Now()
					formatter.Notice("xcode-clt-install",
						"Xcode Command Line Tools are being installed — this can take 10-30 minutes. Please wait...")
				} else if exists && detected && time.Since(lastHeartbeat) >= heartbeatInterval {
					lastHeartbeat = time.Now()
					formatter.Notice("xcode-clt-progress",
						"Xcode Command Line Tools installation is still in progress...")
				} else if !exists && detected {
					formatter.Notice("xcode-clt-complete",
						"Xcode Command Line Tools installation finished.")
					return
				}
			}
		}
	}()

	return cancel
}

// isInstalled checks whether Xcode CLT is already installed by running
// xcode-select -p and checking for a zero exit code.
func isInstalled() bool {
	cmd := exec.Command("xcode-select", "-p")
	return cmd.Run() == nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
