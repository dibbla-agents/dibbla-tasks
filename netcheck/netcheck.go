// Package netcheck provides a lightweight connectivity probe
// to determine if the network is down after a step failure.
package netcheck

import (
	"context"
	"net"
	"sync"
	"time"
)

// IsOffline performs a fast connectivity check by dialing well-known hosts
// in parallel. Returns true if ALL probes fail (network appears down),
// along with a user-friendly message.
func IsOffline(ctx context.Context) (offline bool, message string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	targets := []string{
		"github.com:443",
		"registry.npmjs.org:443",
		"1.1.1.1:53",
	}

	// Probe all targets in parallel — if any succeeds, network is up.
	var wg sync.WaitGroup
	reachable := make(chan struct{}, 1)

	for _, target := range targets {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				select {
				case reachable <- struct{}{}:
				default:
				}
			}
		}(target)
	}

	// Wait for either a success or all probes to finish.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-reachable:
		return false, ""
	case <-done:
		return true, "Your internet connection appears to be down. Please check your connection and try again."
	}
}
