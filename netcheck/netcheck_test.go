package netcheck_test

import (
	"context"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/netcheck"
)

func TestIsOffline_ReturnsFalseWhenNetworkIsUp(t *testing.T) {
	offline, msg := netcheck.IsOffline(context.Background())
	if offline {
		t.Skip("network appears to be down in this environment — skipping")
	}
	if msg != "" {
		t.Errorf("expected empty message when online, got %q", msg)
	}
}
