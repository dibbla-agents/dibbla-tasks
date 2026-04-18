package toolcheck_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/toolcheck"
)

// testPlatform is a mock platform for testing.
type testPlatform struct {
	os string
}

func (p testPlatform) OS() string { return p.os }

func TestChecker_IsAvailable_GitExists(t *testing.T) {
	// git should be available on any development machine
	c := toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor())
	result, err := c.IsAvailable(context.Background(), "git", "git --version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected git to be available")
	}
	if result.Path == "" {
		t.Error("expected git path to be non-empty")
	}
	if !strings.Contains(result.Output, "git version") {
		t.Errorf("expected output to contain 'git version', got %q", result.Output)
	}
}

func TestChecker_IsAvailable_GoExists(t *testing.T) {
	// go should be available since we're building a Go project
	c := toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor())
	result, err := c.IsAvailable(context.Background(), "go", "go version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected go to be available")
	}
	if !strings.Contains(result.Output, "go version") {
		t.Errorf("expected output to contain 'go version', got %q", result.Output)
	}
}

func TestChecker_IsAvailable_NonexistentTool(t *testing.T) {
	c := toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor())
	result, err := c.IsAvailable(context.Background(), "nonexistent_tool_xyz_abc_123", "nonexistent_tool_xyz_abc_123 --version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected nonexistent tool to not be available")
	}
	if result.Path != "" {
		t.Errorf("expected empty path, got %q", result.Path)
	}
}

func TestChecker_IsAvailable_EmptyCheckCmd(t *testing.T) {
	// When check cmd is empty, should fall back to LookPath
	c := toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor())
	result, err := c.IsAvailable(context.Background(), "git", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected git to be available via LookPath")
	}
}

func TestChecker_Install_PlatformSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Use echo as a safe "install" command
	toolDef := model.ToolDef{
		Check: model.PlatformString{Common: "echo ok"},
		Install: map[string]string{
			"darwin":  "echo installed_on_darwin",
			"linux":   "echo installed_on_linux",
			"windows": "echo installed_on_windows",
		},
	}

	platform := testPlatform{os: runtime.GOOS}
	c := toolcheck.NewChecker(platform, executor.NewShellExecutor())

	err := c.Install(context.Background(), "test-tool", toolDef, toolcheck.InstallOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChecker_Install_NoPlatformCommand(t *testing.T) {
	// Use a platform that has no install command
	platform := testPlatform{os: "freebsd"}
	c := toolcheck.NewChecker(platform, executor.NewShellExecutor())

	toolDef := model.ToolDef{
		Check: model.PlatformString{Common: "echo ok"},
		Install: map[string]string{
			"darwin": "echo ok",
			"linux":  "echo ok",
		},
	}

	err := c.Install(context.Background(), "test-tool", toolDef, toolcheck.InstallOpts{})
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "freebsd") {
		t.Errorf("error should mention platform, got: %v", err)
	}
}

func TestChecker_Install_FailingCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	toolDef := model.ToolDef{
		Check: model.PlatformString{Common: "false"},
		Install: map[string]string{
			runtime.GOOS: "false",
		},
	}

	c := toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor())
	err := c.Install(context.Background(), "bad-tool", toolDef, toolcheck.InstallOpts{})
	if err == nil {
		t.Fatal("expected error from failing install command")
	}
}

// --- Privileged Flag Tests ---

// mockExecutor records calls for testing.
type mockExecutor struct {
	lastOpts executor.ExecOpts
	lastCmd  string
}

func (m *mockExecutor) Execute(ctx context.Context, cmd string, opts executor.ExecOpts) (*executor.ExecResult, error) {
	m.lastCmd = cmd
	m.lastOpts = opts
	return &executor.ExecResult{ExitCode: 0}, nil
}

func TestChecker_Install_PrivilegedFlag(t *testing.T) {
	mock := &mockExecutor{}
	platform := testPlatform{os: "darwin"}
	c := toolcheck.NewChecker(platform, mock)

	toolDef := model.ToolDef{
		Check:      model.PlatformString{Common: "brew --version"},
		Privileged: true,
		Install: map[string]string{
			"darwin": "echo install_brew",
		},
	}

	err := c.Install(context.Background(), "brew", toolDef, toolcheck.InstallOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the executor received Privileged=true
	if !mock.lastOpts.Privileged {
		t.Error("expected executor to receive Privileged=true, got false")
	}
	if mock.lastCmd != "echo install_brew" {
		t.Errorf("expected command = %q, got %q", "echo install_brew", mock.lastCmd)
	}
}

func TestChecker_Install_NonPrivilegedFlag(t *testing.T) {
	mock := &mockExecutor{}
	platform := testPlatform{os: "darwin"}
	c := toolcheck.NewChecker(platform, mock)

	toolDef := model.ToolDef{
		Check: model.PlatformString{Common: "go version"},
		Install: map[string]string{
			"darwin": "brew install go",
		},
	}

	err := c.Install(context.Background(), "go", toolDef, toolcheck.InstallOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the executor received Privileged=false (default)
	if mock.lastOpts.Privileged {
		t.Error("expected executor to receive Privileged=false, got true")
	}
}
