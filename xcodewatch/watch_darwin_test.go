//go:build darwin

package xcodewatch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// mockFormatter records Notice calls for assertions.
type mockFormatter struct {
	mu      sync.Mutex
	notices []noticeCall
}

type noticeCall struct {
	title   string
	message string
}

func (m *mockFormatter) StartStep(string, string)           {}
func (m *mockFormatter) EndStep(string, string, bool)       {}
func (m *mockFormatter) SkipStep(string, string, string)    {}
func (m *mockFormatter) Error(string, string)               {}
func (m *mockFormatter) Warning(string, string)             {}
func (m *mockFormatter) Debug(string)                       {}
func (m *mockFormatter) Stdout(string, string)              {}
func (m *mockFormatter) Stderr(string, string)              {}
func (m *mockFormatter) Notice(title, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notices = append(m.notices, noticeCall{title: title, message: message})
}

func (m *mockFormatter) getNotices() []noticeCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]noticeCall, len(m.notices))
	copy(cp, m.notices)
	return cp
}

// Verify mockFormatter satisfies model.Formatter.
var _ model.Formatter = (*mockFormatter)(nil)

func TestWatch_NoSentinelFile(t *testing.T) {
	f := &mockFormatter{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	sentinel := filepath.Join(t.TempDir(), "sentinel")

	cancel := watchWithConfig(ctx, f, watchConfig{
		sentinelPath:      sentinel,
		pollInterval:      20 * time.Millisecond,
		heartbeatInterval: 100 * time.Millisecond,
		skipInstallCheck:  true,
	})
	defer cancel()

	// Let a few poll cycles pass
	time.Sleep(80 * time.Millisecond)

	notices := f.getNotices()
	if len(notices) != 0 {
		t.Fatalf("expected no notices when sentinel absent, got %d", len(notices))
	}
}

func TestWatch_DetectsInstall(t *testing.T) {
	f := &mockFormatter{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	sentinel := filepath.Join(t.TempDir(), "sentinel")

	cancel := watchWithConfig(ctx, f, watchConfig{
		sentinelPath:      sentinel,
		pollInterval:      20 * time.Millisecond,
		heartbeatInterval: 5 * time.Second,
		skipInstallCheck:  true,
	})
	defer cancel()

	// Create sentinel file to simulate CLT install starting
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(80 * time.Millisecond)

	notices := f.getNotices()
	if len(notices) == 0 {
		t.Fatal("expected at least one notice after sentinel appears")
	}
	if notices[0].title != "xcode-clt-install" {
		t.Fatalf("expected first notice title 'xcode-clt-install', got %q", notices[0].title)
	}
}

func TestWatch_DetectsCompletion(t *testing.T) {
	f := &mockFormatter{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	sentinel := filepath.Join(t.TempDir(), "sentinel")

	// Create sentinel before starting watcher
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cancel := watchWithConfig(ctx, f, watchConfig{
		sentinelPath:      sentinel,
		pollInterval:      20 * time.Millisecond,
		heartbeatInterval: 5 * time.Second,
		skipInstallCheck:  true,
	})
	defer cancel()

	// Wait for detection
	time.Sleep(80 * time.Millisecond)

	// Remove sentinel to simulate completion
	os.Remove(sentinel)

	// Wait for completion detection
	time.Sleep(80 * time.Millisecond)

	notices := f.getNotices()
	var hasInstall, hasComplete bool
	for _, n := range notices {
		if n.title == "xcode-clt-install" {
			hasInstall = true
		}
		if n.title == "xcode-clt-complete" {
			hasComplete = true
		}
	}
	if !hasInstall {
		t.Error("expected xcode-clt-install notice")
	}
	if !hasComplete {
		t.Error("expected xcode-clt-complete notice")
	}
}

func TestWatch_HeartbeatEmitted(t *testing.T) {
	f := &mockFormatter{}
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Short heartbeat interval for testing
	cancel := watchWithConfig(ctx, f, watchConfig{
		sentinelPath:      sentinel,
		pollInterval:      20 * time.Millisecond,
		heartbeatInterval: 60 * time.Millisecond,
		skipInstallCheck:  true,
	})
	defer cancel()

	// Wait long enough for initial detection + at least one heartbeat
	time.Sleep(150 * time.Millisecond)

	notices := f.getNotices()
	var progressCount int
	for _, n := range notices {
		if n.title == "xcode-clt-progress" {
			progressCount++
		}
	}
	if progressCount == 0 {
		t.Error("expected at least one xcode-clt-progress heartbeat notice")
	}
}

func TestWatch_ContextCancellation(t *testing.T) {
	f := &mockFormatter{}
	ctx, ctxCancel := context.WithCancel(context.Background())

	sentinel := filepath.Join(t.TempDir(), "sentinel")

	cancel := watchWithConfig(ctx, f, watchConfig{
		sentinelPath:      sentinel,
		pollInterval:      20 * time.Millisecond,
		heartbeatInterval: 5 * time.Second,
		skipInstallCheck:  true,
	})
	defer cancel()

	// Cancel context — goroutine should exit cleanly
	ctxCancel()

	time.Sleep(60 * time.Millisecond)

	// Create sentinel after cancellation — should NOT emit notices
	os.WriteFile(sentinel, []byte{}, 0644)
	time.Sleep(60 * time.Millisecond)

	notices := f.getNotices()
	if len(notices) != 0 {
		t.Fatalf("expected no notices after cancellation, got %d", len(notices))
	}
}
