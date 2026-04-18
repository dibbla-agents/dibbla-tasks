//go:build !windows

package executor_test

import (
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/executor"
)

func TestBuildTerminalAppleScript(t *testing.T) {
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
	result := executor.BuildTerminalAppleScript(`cd "/path with spaces" && echo "hello"`)
	if !strings.Contains(result, "path with spaces") {
		t.Errorf("expected path with spaces to be preserved, got %q", result)
	}
}
