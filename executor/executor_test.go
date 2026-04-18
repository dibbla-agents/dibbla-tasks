package executor_test

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/executor"
)

func TestShellExecutor_EchoCommand(t *testing.T) {
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	result, err := e.Execute(context.Background(), "echo hello", executor.ExecOpts{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Errorf("stdout = %q, want %q", got, "hello")
	}
}

func TestShellExecutor_FailingCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows — 'false' is not a standard command")
	}
	e := executor.NewShellExecutor()

	result, err := e.Execute(context.Background(), "false", executor.ExecOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code from 'false'")
	}
}

func TestShellExecutor_StderrCapture(t *testing.T) {
	e := executor.NewShellExecutor()
	var stderr bytes.Buffer

	cmd := "echo error_output >&2"
	if runtime.GOOS == "windows" {
		cmd = "echo error_output 1>&2"
	}

	result, err := e.Execute(context.Background(), cmd, executor.ExecOpts{
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(stderr.String()); got != "error_output" {
		t.Errorf("stderr = %q, want %q", got, "error_output")
	}
}

func TestShellExecutor_EnvVars(t *testing.T) {
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	cmd := "echo $MY_TEST_VAR"
	if runtime.GOOS == "windows" {
		cmd = "echo %MY_TEST_VAR%"
	}

	result, err := e.Execute(context.Background(), cmd, executor.ExecOpts{
		Env:    map[string]string{"MY_TEST_VAR": "test_value_123"},
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "test_value_123" {
		t.Errorf("stdout = %q, want %q", got, "test_value_123")
	}
}

func TestShellExecutor_WorkingDir(t *testing.T) {
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	cmd := "pwd"
	if runtime.GOOS == "windows" {
		cmd = "cd"
	}

	result, err := e.Execute(context.Background(), cmd, executor.ExecOpts{
		Dir:    "/tmp",
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	got := strings.TrimSpace(stdout.String())
	// On macOS /tmp is a symlink to /private/tmp
	if got != "/tmp" && got != "/private/tmp" {
		t.Errorf("working dir = %q, want /tmp or /private/tmp", got)
	}
}

func TestShellExecutor_ContextCancellation(t *testing.T) {
	e := executor.NewShellExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := e.Execute(ctx, "sleep 60", executor.ExecOpts{})
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestShellExecutor_MultiLineCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("multi-line commands differ on windows")
	}
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	result, err := e.Execute(context.Background(), "echo line1 && echo line2", executor.ExecOpts{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("output lines = %d, want 2", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("output = %v, want [line1 line2]", lines)
	}
}

// --- Login Shell Environment Tests ---

func TestLoginEnvPATH_NonEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login env resolution not applicable on windows")
	}
	path := executor.LoginEnvPATH()
	if path == "" {
		t.Error("LoginEnvPATH() returned empty string; expected enriched PATH")
	}
	// The PATH should contain at least /usr/bin
	if !strings.Contains(path, "/usr/bin") {
		t.Errorf("LoginEnvPATH() = %q, expected to contain /usr/bin", path)
	}
}

func TestShellExecutor_InheritsEnrichedPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login env resolution not applicable on windows")
	}
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	// The executor should have an enriched PATH even if the process
	// PATH is minimal. We verify by checking that the command sees
	// a PATH containing /usr/bin at minimum.
	result, err := e.Execute(context.Background(), "echo $PATH", executor.ExecOpts{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	got := strings.TrimSpace(stdout.String())
	if !strings.Contains(got, "/usr/bin") {
		t.Errorf("command PATH = %q, expected to contain /usr/bin", got)
	}
}

func TestShellExecutor_StepEnvOverridesLoginEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login env resolution not applicable on windows")
	}
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	// Step-specific env should override login env (highest priority).
	customPath := "/custom/test/path"
	result, err := e.Execute(context.Background(), "echo $PATH", executor.ExecOpts{
		Env:    map[string]string{"PATH": customPath},
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	got := strings.TrimSpace(stdout.String())
	if got != customPath {
		t.Errorf("PATH = %q, want %q (step env should override)", got, customPath)
	}
}

// --- Privileged Command Building Tests ---

func TestBuildPrivilegedOsascript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("osascript not available on windows")
	}

	result := executor.BuildPrivilegedOsascript("/tmp/test-script.sh")
	expected := `do shell script "bash '/tmp/test-script.sh'" with administrator privileges`
	if result != expected {
		t.Errorf("BuildPrivilegedOsascript() = %q, want %q", result, expected)
	}
}

// --- Background Mode Tests ---

func TestShellExecutor_BackgroundModeFastCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	e := executor.NewShellExecutor()
	var stdout bytes.Buffer

	// A fast command should complete within the 2-second grace period.
	result, err := e.Execute(context.Background(), "echo background_hello", executor.ExecOpts{
		Stdout: &stdout,
		Mode:   executor.ExecModeBackground,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(stdout.String()); got != "background_hello" {
		t.Errorf("stdout = %q, want %q", got, "background_hello")
	}
}

func TestShellExecutor_BackgroundModeFailingCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	e := executor.NewShellExecutor()

	// A command that fails immediately should return non-zero exit code.
	result, err := e.Execute(context.Background(), "false", executor.ExecOpts{
		Mode: executor.ExecModeBackground,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code from 'false' in background mode")
	}
}

func TestShellExecutor_BackgroundModeLongRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	e := executor.NewShellExecutor()

	// A long-running command should succeed after the 2-second grace period.
	result, err := e.Execute(context.Background(), "sleep 60", executor.ExecOpts{
		Mode: executor.ExecModeBackground,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0 (background process still running)", result.ExitCode)
	}
}

// --- Terminal Mode Tests ---

func TestBuildTerminalAppleScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("AppleScript not available on windows")
	}
	result := executor.BuildTerminalAppleScript("cd '/tmp' && npm run dev")
	if !strings.Contains(result, "do script") {
		t.Errorf("expected AppleScript to contain 'do script', got %q", result)
	}
	if !strings.Contains(result, "npm run dev") {
		t.Errorf("expected AppleScript to contain the command, got %q", result)
	}
	if !strings.Contains(result, "activate") {
		t.Errorf("expected AppleScript to contain 'activate', got %q", result)
	}
}

func TestBuildTerminalAppleScript_SpecialChars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("AppleScript not available on windows")
	}
	result := executor.BuildTerminalAppleScript(`cd "/path with spaces" && echo "hello"`)
	if !strings.Contains(result, "path with spaces") {
		t.Errorf("expected path with spaces to be preserved, got %q", result)
	}
}

func TestBuildPrivilegedOsascript_SpecialChars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("osascript not available on windows")
	}

	// Test with a path containing spaces (common on macOS)
	result := executor.BuildPrivilegedOsascript("/tmp/my script.sh")
	expected := `do shell script "bash '/tmp/my script.sh'" with administrator privileges`
	if result != expected {
		t.Errorf("BuildPrivilegedOsascript() = %q, want %q", result, expected)
	}
}
