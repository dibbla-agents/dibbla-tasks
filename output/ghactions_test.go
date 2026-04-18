package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/output"
)

func TestGHActionsFormatter_StartStep(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.StartStep("build", "Build Project")
	if got := buf.String(); got != "::group::Build Project\n" {
		t.Errorf("got %q, want %q", got, "::group::Build Project\n")
	}
}

func TestGHActionsFormatter_EndStep(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.EndStep("build", "Build Project", true)
	if got := buf.String(); got != "::endgroup::\n" {
		t.Errorf("got %q, want %q", got, "::endgroup::\n")
	}
}

func TestGHActionsFormatter_Error(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.Error("Step Failed", "exit code 1")
	want := "::error title=Step Failed::exit code 1\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGHActionsFormatter_Warning(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.Warning("Deprecation", "this API is deprecated")
	want := "::warning title=Deprecation::this API is deprecated\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGHActionsFormatter_Notice(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.Notice("Tool Check", "go is installed")
	want := "::notice title=Tool Check::go is installed\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGHActionsFormatter_Debug(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.Debug("verbose info here")
	want := "::debug::verbose info here\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGHActionsFormatter_Stdout(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)
	f.Stdout("build", "compiling main.go...")
	want := "compiling main.go...\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGHActionsFormatter_FullStep(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)

	f.StartStep("build", "Build Project")
	f.Debug("working dir: /tmp")
	f.Stdout("build", "compiling...")
	f.Notice("Build", "compiled successfully")
	f.EndStep("build", "Build Project", true)

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")

	expected := []string{
		"::group::Build Project",
		"::debug::working dir: /tmp",
		"compiling...",
		"::notice title=Build::compiled successfully",
		"::endgroup::",
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(expected), out)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestGHActionsFormatter_FailedStep(t *testing.T) {
	var buf bytes.Buffer
	f := output.NewGHActionsFormatter(&buf)

	f.StartStep("test", "Run Tests")
	f.Stderr("test", "FAIL main_test.go")
	f.Error("Step Failed", "Step \"Run Tests\" failed with exit code 1")
	f.EndStep("test", "Run Tests", false)

	out := buf.String()
	if !strings.Contains(out, "::group::Run Tests") {
		t.Error("missing group start")
	}
	if !strings.Contains(out, "::error title=Step Failed") {
		t.Error("missing error annotation")
	}
	if !strings.Contains(out, "::endgroup::") {
		t.Error("missing group end")
	}
}
