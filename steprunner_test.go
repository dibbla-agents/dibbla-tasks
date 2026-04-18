package steprunner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks"
	"github.com/dibbla-agents/dibbla-tasks/output"
)

// testdataPath returns the absolute path to a testdata file.
func testdataPath(name string) string {
	// Tests run with the package directory as cwd
	return filepath.Join("testdata", name)
}

// --- Integration: Full Pipeline Tests ---

func TestIntegration_BasicSteps(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_basic.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}

	out := buf.String()

	// Verify GH Actions output format
	if !strings.Contains(out, "::group::Say Hello") {
		t.Error("missing group for 'Say Hello'")
	}
	if !strings.Contains(out, "hello_world") {
		t.Error("missing echo output 'hello_world'")
	}
	if !strings.Contains(out, "::group::Say Goodbye") {
		t.Error("missing group for 'Say Goodbye'")
	}
	if !strings.Contains(out, "goodbye_world") {
		t.Error("missing echo output 'goodbye_world'")
	}
	if strings.Count(out, "::endgroup::") != 2 {
		t.Errorf("expected 2 endgroup markers, got %d", strings.Count(out, "::endgroup::"))
	}
}

func TestIntegration_ToolChecks(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_tools.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success — git and go should be available")
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(result.Steps))
	}

	// All steps should succeed
	for _, sr := range result.Steps {
		if sr.Status != steprunner.StepStatusSuccess {
			t.Errorf("step %q status = %q, want success", sr.StepID, sr.Status)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "::notice title=Tool Check") {
		t.Error("missing tool check notice")
	}
	if !strings.Contains(out, "All tools verified") {
		t.Error("missing final step output")
	}
}

func TestIntegration_DependencyChain(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_deps.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Steps) != 4 {
		t.Fatalf("expected 4 step results, got %d", len(result.Steps))
	}

	// step-a must be first, step-d must be last
	if result.Steps[0].StepID != "step-a" {
		t.Errorf("first step = %q, want 'step-a'", result.Steps[0].StepID)
	}
	if result.Steps[3].StepID != "step-d" {
		t.Errorf("last step = %q, want 'step-d'", result.Steps[3].StepID)
	}

	out := buf.String()
	// Verify all steps appear in output
	for _, id := range []string{"step_a", "step_b", "step_c", "step_d"} {
		if !strings.Contains(out, id) {
			t.Errorf("missing output for %s", id)
		}
	}
}

func TestIntegration_FullPipeline(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_full.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Steps) != 7 {
		t.Fatalf("expected 7 step results, got %d", len(result.Steps))
	}

	out := buf.String()

	// Verify tool checks
	if !strings.Contains(out, "::notice title=Tool Check") {
		t.Error("missing tool check notices")
	}

	// Verify command outputs
	expectedOutputs := []string{
		"Setting up environment...",
		"Building project...",
		"Running tests... all passed",
		"Linting... no issues found",
		"All steps completed successfully",
	}
	for _, expected := range expectedOutputs {
		if !strings.Contains(out, expected) {
			t.Errorf("missing expected output: %q", expected)
		}
	}

	// Verify GH Actions structure
	groupCount := strings.Count(out, "::group::")
	endGroupCount := strings.Count(out, "::endgroup::")
	if groupCount != endGroupCount {
		t.Errorf("group/endgroup mismatch: %d groups, %d endgroups", groupCount, endGroupCount)
	}
	if groupCount != 7 {
		t.Errorf("expected 7 groups, got %d", groupCount)
	}
}

func TestIntegration_InvalidVersion(t *testing.T) {
	_, err := steprunner.Run(context.Background(), testdataPath("invalid_version.yaml"))
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
	if !errors.Is(err, steprunner.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestIntegration_InvalidMissingName(t *testing.T) {
	_, err := steprunner.Run(context.Background(), testdataPath("invalid_missing_name.yaml"))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !errors.Is(err, steprunner.ErrMissingStepName) {
		t.Errorf("error = %v, want ErrMissingStepName", err)
	}
}

func TestIntegration_CyclicDependency(t *testing.T) {
	_, err := steprunner.Run(context.Background(), testdataPath("invalid_cycle.yaml"))
	if err == nil {
		t.Fatal("expected error for cyclic dependency")
	}
	if !errors.Is(err, steprunner.ErrCyclicDependency) {
		t.Errorf("error = %v, want ErrCyclicDependency", err)
	}
}

func TestIntegration_FileNotFound(t *testing.T) {
	_, err := steprunner.Run(context.Background(), "nonexistent_file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- Preview Tests ---

func TestPreview_BasicFile(t *testing.T) {
	result, err := steprunner.Preview(testdataPath("valid_basic.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.StepSummaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(result.StepSummaries))
	}
	if result.StepSummaries[0].ID != "hello" {
		t.Errorf("first step id = %q, want %q", result.StepSummaries[0].ID, "hello")
	}
	if result.StepSummaries[0].Name != "Say Hello" {
		t.Errorf("first step name = %q, want %q", result.StepSummaries[0].Name, "Say Hello")
	}
	if result.StepSummaries[0].Type != steprunner.StepTypeCommand {
		t.Errorf("first step type = %q, want %q", result.StepSummaries[0].Type, steprunner.StepTypeCommand)
	}
}

func TestPreview_FullFile(t *testing.T) {
	result, err := steprunner.Preview(testdataPath("valid_full.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ExecutionOrder) != 7 {
		t.Fatalf("expected 7 steps in execution order, got %d", len(result.ExecutionOrder))
	}

	// verify-git and verify-go must come before setup
	setupIdx := -1
	for i, id := range result.ExecutionOrder {
		if id == "setup" {
			setupIdx = i
		}
	}
	for i, id := range result.ExecutionOrder {
		if id == "verify-git" || id == "verify-go" {
			if i >= setupIdx {
				t.Errorf("tool check %q (index %d) should come before setup (index %d)", id, i, setupIdx)
			}
		}
	}

	// report must be last
	if result.ExecutionOrder[6] != "report" {
		t.Errorf("last step = %q, want 'report'", result.ExecutionOrder[6])
	}
}

func TestPreview_CyclicDependency(t *testing.T) {
	_, err := steprunner.Preview(testdataPath("invalid_cycle.yaml"))
	if err == nil {
		t.Fatal("expected error for cyclic dependency in preview")
	}
	if !errors.Is(err, steprunner.ErrCyclicDependency) {
		t.Errorf("error = %v, want ErrCyclicDependency", err)
	}
}

func TestPreview_InvalidFile(t *testing.T) {
	_, err := steprunner.Preview(testdataPath("invalid_version.yaml"))
	if err == nil {
		t.Fatal("expected error for invalid version in preview")
	}
}

func TestPreviewString(t *testing.T) {
	yaml := `
version: "1"
steps:
  - id: test
    name: "Test"
    type: command
    run: "echo test"
`
	result, err := steprunner.PreviewString(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.StepSummaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(result.StepSummaries))
	}
}

// --- Callback Formatter Integration Test ---

func TestIntegration_CallbackFormatter(t *testing.T) {
	var events []steprunner.StepEvent
	cb := output.NewCallbackFormatter(func(e steprunner.StepEvent) {
		events = append(events, e)
	})

	result, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: hello
    name: "Say Hello"
    type: command
    run: "echo callback_test"
`, steprunner.WithFormatter(cb))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Should have at least: StartStep, stdout line, EndStep
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// First event should be StartStep with Running status and EventTypeStatus
	if events[0].Type != steprunner.EventTypeStatus {
		t.Errorf("first event type = %q, want %q", events[0].Type, steprunner.EventTypeStatus)
	}
	if events[0].Status != steprunner.StepStatusRunning {
		t.Errorf("first event status = %q, want %q", events[0].Status, steprunner.StepStatusRunning)
	}
	if events[0].StepID != "hello" {
		t.Errorf("first event stepID = %q, want %q", events[0].StepID, "hello")
	}

	// Last event should be EndStep with Success status and EventTypeStatus
	last := events[len(events)-1]
	if last.Type != steprunner.EventTypeStatus {
		t.Errorf("last event type = %q, want %q", last.Type, steprunner.EventTypeStatus)
	}
	if last.Status != steprunner.StepStatusSuccess {
		t.Errorf("last event status = %q, want %q", last.Status, steprunner.StepStatusSuccess)
	}

	// Should contain a stdout event with the output line
	hasOutput := false
	for _, e := range events {
		if e.Type == steprunner.EventTypeStdout && strings.Contains(e.Output, "callback_test") {
			hasOutput = true
			break
		}
	}
	if !hasOutput {
		t.Error("no stdout event contained 'callback_test' output")
	}
}

// --- Multi Formatter Integration Test ---

func TestIntegration_MultiFormatter(t *testing.T) {
	var ghBuf bytes.Buffer
	var events []steprunner.StepEvent

	ghFmt := output.NewGHActionsFormatter(&ghBuf)
	cbFmt := output.NewCallbackFormatter(func(e steprunner.StepEvent) {
		events = append(events, e)
	})
	multi := output.NewMultiFormatter(ghFmt, cbFmt)

	result, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: test
    name: "Multi Test"
    type: command
    run: "echo multi_output"
`, steprunner.WithFormatter(multi))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// GH Actions output should have the group
	ghOut := ghBuf.String()
	if !strings.Contains(ghOut, "::group::Multi Test") {
		t.Error("GH Actions formatter missing group")
	}
	if !strings.Contains(ghOut, "multi_output") {
		t.Error("GH Actions formatter missing command output")
	}

	// Callback should have events
	if len(events) < 3 {
		t.Fatalf("expected at least 3 callback events, got %d", len(events))
	}
}

// --- Working Directory Test ---

func TestIntegration_WorkingDir(t *testing.T) {
	// Use a well-known directory that exists and has a predictable resolved path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}
	var buf bytes.Buffer

	result, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: pwd
    name: "Print Working Dir"
    type: command
    run: "pwd"
`,
		steprunner.WithOutputStream(&buf),
		steprunner.WithWorkDir(homeDir),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	// The output should contain the home directory path
	if !strings.Contains(out, homeDir) {
		t.Errorf("output should contain home dir %q, got: %s", homeDir, out)
	}
}

func TestIntegration_RelativeWorkingDir(t *testing.T) {
	// Create a temp directory with a subdirectory to simulate a project layout.
	rootDir := t.TempDir()
	subDir := filepath.Join(rootDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	var buf bytes.Buffer

	// The step uses working_dir: "./subdir" which should resolve to rootDir/subdir.
	result, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: pwd-sub
    name: "Print Subdir"
    type: command
    run: "pwd"
    working_dir: "./subdir"
`,
		steprunner.WithOutputStream(&buf),
		steprunner.WithWorkDir(rootDir),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got steps: %+v", result.Steps)
	}

	out := buf.String()
	// The resolved path of the subdir should appear in the output.
	resolvedSubDir, _ := filepath.EvalSymlinks(subDir)
	if !strings.Contains(out, resolvedSubDir) && !strings.Contains(out, subDir) {
		t.Errorf("output should contain subdir path %q (or %q), got: %s", subDir, resolvedSubDir, out)
	}
}

func TestIntegration_AbsoluteWorkingDir(t *testing.T) {
	// Verify that an absolute working_dir is used as-is, not joined to workDir.
	absDir := t.TempDir()
	anotherDir := t.TempDir() // workDir that should NOT appear in output

	var buf bytes.Buffer

	result, err := steprunner.RunString(context.Background(), fmt.Sprintf(`
version: "1"
steps:
  - id: pwd-abs
    name: "Print Abs Dir"
    type: command
    run: "pwd"
    working_dir: %q
`, absDir),
		steprunner.WithOutputStream(&buf),
		steprunner.WithWorkDir(anotherDir),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got steps: %+v", result.Steps)
	}

	out := buf.String()
	resolvedAbsDir, _ := filepath.EvalSymlinks(absDir)
	if !strings.Contains(out, resolvedAbsDir) && !strings.Contains(out, absDir) {
		t.Errorf("output should contain abs dir %q (or %q), got: %s", absDir, resolvedAbsDir, out)
	}
}

// --- Environment Variable Test ---

func TestIntegration_EnvVars(t *testing.T) {
	var buf bytes.Buffer

	result, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: env
    name: "Echo Env"
    type: command
    run: "echo $CUSTOM_VAR"
    env:
      CUSTOM_VAR: "from_dibbla_task"
`,
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if !strings.Contains(buf.String(), "from_dibbla_task") {
		t.Error("missing custom env var in output")
	}
}

// --- Pipeline-level Environment Variable Tests ---

func TestIntegration_EnvVarDefaults(t *testing.T) {
	var buf bytes.Buffer

	result, err := steprunner.Run(context.Background(), testdataPath("valid_env_defaults.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	// Verify default values flow through to the command step.
	if !strings.Contains(out, "APP=MyApp") {
		t.Errorf("expected APP=MyApp in output, got: %s", out)
	}
	if !strings.Contains(out, "PORT=8080") {
		t.Errorf("expected PORT=8080 in output, got: %s", out)
	}
	if !strings.Contains(out, "LOG=info") {
		t.Errorf("expected LOG=info in output, got: %s", out)
	}
}

func TestIntegration_EnvVarWithOverrides(t *testing.T) {
	var buf bytes.Buffer

	result, err := steprunner.Run(context.Background(), testdataPath("valid_env_defaults.yaml"),
		steprunner.WithOutputStream(&buf),
		steprunner.WithEnv(map[string]string{
			"APP_NAME":  "OverriddenApp",
			"LOG_LEVEL": "debug",
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	if !strings.Contains(out, "APP=OverriddenApp") {
		t.Errorf("expected APP=OverriddenApp in output, got: %s", out)
	}
	if !strings.Contains(out, "LOG=debug") {
		t.Errorf("expected LOG=debug in output, got: %s", out)
	}
}

func TestIntegration_RequiredEnvMissing(t *testing.T) {
	_, err := steprunner.Run(context.Background(), testdataPath("valid_env.yaml"))
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	if !errors.Is(err, steprunner.ErrMissingRequiredEnv) {
		t.Errorf("error = %v, want ErrMissingRequiredEnv", err)
	}
}

func TestIntegration_RequiredEnvProvided(t *testing.T) {
	var buf bytes.Buffer

	result, err := steprunner.Run(context.Background(), testdataPath("valid_env.yaml"),
		steprunner.WithOutputStream(&buf),
		steprunner.WithEnv(map[string]string{
			"API_KEY": "sk-test-12345",
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	if !strings.Contains(out, "DB=postgres://localhost:5432/mydb") {
		t.Errorf("expected default DATABASE_URL in output, got: %s", out)
	}
	if !strings.Contains(out, "LOG=info") {
		t.Errorf("expected default LOG_LEVEL in output, got: %s", out)
	}
}

func TestIntegration_WriteEnvCreatesFile(t *testing.T) {
	dir := t.TempDir()

	// Create a task file in the temp directory.
	taskContent := `version: "1"
env:
  MY_KEY:
    description: "Test key"
    default: "test_value"
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
`
	taskPath := filepath.Join(dir, "dibbla-task.yaml")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), taskPath,
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// The .env file should be in the same directory as the task file.
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("expected .env to exist: %v", err)
	}
	content := string(envData)
	if !strings.Contains(content, "MY_KEY=test_value") {
		t.Errorf("expected MY_KEY=test_value in .env, got: %s", content)
	}
	if !strings.Contains(content, "# Test key") {
		t.Errorf("expected description comment, got: %s", content)
	}
}

func TestIntegration_WorkDirDefaultsToFileDir(t *testing.T) {
	dir := t.TempDir()

	taskContent := `version: "1"
steps:
  - id: pwd
    name: "Print Working Dir"
    type: command
    run: "pwd"
`
	taskPath := filepath.Join(dir, "dibbla-task.yaml")
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		t.Fatalf("failed to write task file: %v", err)
	}

	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), taskPath,
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	// The output should contain the temp directory path.
	if !strings.Contains(out, dir) {
		t.Errorf("working dir should default to file dir %q, got: %s", dir, out)
	}
}

func TestPreview_WithEnvVars(t *testing.T) {
	result, err := steprunner.PreviewString(`
version: "1"
env:
  DATABASE_URL:
    description: "PostgreSQL connection string"
    default: "postgres://localhost:5432/mydb"
  API_KEY:
    description: "Third-party API key"
    required: true
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.EnvVars) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(result.EnvVars))
	}

	// EnvVars should be sorted by name.
	if result.EnvVars[0].Name != "API_KEY" {
		t.Errorf("first env var = %q, want %q", result.EnvVars[0].Name, "API_KEY")
	}
	if result.EnvVars[1].Name != "DATABASE_URL" {
		t.Errorf("second env var = %q, want %q", result.EnvVars[1].Name, "DATABASE_URL")
	}

	// Verify metadata.
	apiKey := result.EnvVars[0]
	if apiKey.Description != "Third-party API key" {
		t.Errorf("API_KEY description = %q", apiKey.Description)
	}
	if !apiKey.Required {
		t.Error("API_KEY should be required")
	}

	dbURL := result.EnvVars[1]
	if dbURL.Default != "postgres://localhost:5432/mydb" {
		t.Errorf("DATABASE_URL default = %q", dbURL.Default)
	}
}

func TestPreview_NoEnvVars(t *testing.T) {
	result, err := steprunner.PreviewString(`
version: "1"
steps:
  - id: test
    name: "Test"
    type: command
    run: "echo test"
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.EnvVars) != 0 {
		t.Errorf("expected 0 env vars, got %d", len(result.EnvVars))
	}
}

// --- Verify GH Actions Output Format Compliance ---

func TestIntegration_GHActionsOutputFormat(t *testing.T) {
	var buf bytes.Buffer

	_, err := steprunner.RunString(context.Background(), `
version: "1"
steps:
  - id: pass
    name: "Passing Step"
    type: command
    run: "echo output_line"
  - id: fail
    name: "Failing Step"
    type: command
    run: "exit 1"
    continue_on_error: true
`,
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	lines := strings.Split(out, "\n")

	// Verify every :: prefixed line is a valid workflow command
	validPrefixes := []string{
		"::group::",
		"::endgroup::",
		"::error ",
		"::warning ",
		"::notice ",
		"::debug::",
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "::") {
			continue // raw output line, ok
		}
		valid := false
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(line, prefix) {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("invalid workflow command: %q", line)
		}
	}

	// Verify structure: groups are properly opened and closed
	groupDepth := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "::group::") {
			groupDepth++
		}
		if strings.HasPrefix(line, "::endgroup::") {
			groupDepth--
		}
		if groupDepth < 0 {
			t.Error("endgroup without matching group")
		}
	}
	if groupDepth != 0 {
		t.Errorf("unmatched groups: depth = %d at end", groupDepth)
	}
}

// --- Platform Support Integration Tests ---

func TestIntegration_PlatformRunMap(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_platform_run.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}

	out := buf.String()
	// On macOS the darwin command should have run.
	if !strings.Contains(out, "installed_on_darwin") {
		t.Error("expected darwin install output on macOS")
	}
	if !strings.Contains(out, "build_complete") {
		t.Error("expected build step output")
	}
}

func TestIntegration_PlatformFilter(t *testing.T) {
	var buf bytes.Buffer
	result, err := steprunner.Run(context.Background(), testdataPath("valid_platform_filter.yaml"),
		steprunner.WithOutputStream(&buf),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()

	// Common and mac-only and final should run on macOS.
	if !strings.Contains(out, "common_setup") {
		t.Error("expected common_setup output")
	}
	if !strings.Contains(out, "mac_step") {
		t.Error("expected mac_step output on darwin")
	}
	if !strings.Contains(out, "final_step") {
		t.Error("expected final_step output")
	}

	// Linux/Windows steps should NOT run.
	if strings.Contains(out, "linux_step") {
		t.Error("linux_step should not run on darwin")
	}
	if strings.Contains(out, "windows_step") {
		t.Error("windows_step should not run on darwin")
	}

	// On darwin: common-setup, mac-only, final = 3 steps
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 step results on darwin, got %d", len(result.Steps))
	}
}

func TestPreview_PlatformMetadata(t *testing.T) {
	result, err := steprunner.PreviewString(`
version: "1"
steps:
  - id: common
    name: "Common"
    type: command
    run: "echo hello"
  - id: mac-only
    name: "Mac Only"
    type: command
    platforms: ["darwin"]
    run: "echo mac"
  - id: cross-platform
    name: "Cross Platform"
    type: command
    run:
      darwin: "echo darwin"
      linux: "echo linux"
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Preview returns ALL steps regardless of current platform.
	if len(result.StepSummaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(result.StepSummaries))
	}

	// common step has no platforms restriction.
	if len(result.StepSummaries[0].Platforms) != 0 {
		t.Errorf("common step platforms = %v, want empty", result.StepSummaries[0].Platforms)
	}

	// mac-only step has platforms: ["darwin"].
	if len(result.StepSummaries[1].Platforms) != 1 || result.StepSummaries[1].Platforms[0] != "darwin" {
		t.Errorf("mac-only platforms = %v, want [darwin]", result.StepSummaries[1].Platforms)
	}
}

func TestPreviewString_PlatformRunMap(t *testing.T) {
	result, err := steprunner.PreviewString(`
version: "1"
steps:
  - id: install
    name: "Install"
    type: command
    run:
      darwin: "brew install foo"
      linux: "apt install foo"
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.StepSummaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(result.StepSummaries))
	}
	// The raw StepFile should contain the platform map.
	step := result.File.Steps[0]
	if step.Run.PerPlatform["darwin"] != "brew install foo" {
		t.Errorf("Run.PerPlatform[darwin] = %q", step.Run.PerPlatform["darwin"])
	}
}

// --- Privileged Tool Preview Test ---

func TestPreview_PrivilegedTool(t *testing.T) {
	result, err := steprunner.Preview(testdataPath("valid_privileged_tool.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 steps: check-brew, check-go, build
	if len(result.StepSummaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(result.StepSummaries))
	}

	// Verify brew tool is marked as privileged in the parsed file.
	brewTool, ok := result.File.Tools["brew"]
	if !ok {
		t.Fatal("tool 'brew' not found in parsed file")
	}
	if !brewTool.Privileged {
		t.Error("expected brew tool to have privileged = true")
	}

	// Verify go tool is NOT privileged.
	goTool, ok := result.File.Tools["go"]
	if !ok {
		t.Fatal("tool 'go' not found in parsed file")
	}
	if goTool.Privileged {
		t.Error("expected go tool to have privileged = false")
	}

	// Verify check-brew step exists and references brew tool.
	if result.StepSummaries[0].Tool != "brew" {
		t.Errorf("first step tool = %q, want %q", result.StepSummaries[0].Tool, "brew")
	}
}

func TestPreview_StepModes(t *testing.T) {
	result, err := steprunner.Preview(testdataPath("valid_step_modes.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.StepSummaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(result.StepSummaries))
	}

	// Build step should have default (empty) mode.
	buildSummary := findSummary(result.StepSummaries, "build")
	if buildSummary == nil {
		t.Fatal("build step not found")
	}
	if buildSummary.Mode != "" {
		t.Errorf("build mode = %q, want empty (foreground)", buildSummary.Mode)
	}

	// Server step should have background mode.
	serverSummary := findSummary(result.StepSummaries, "run-server")
	if serverSummary == nil {
		t.Fatal("run-server step not found")
	}
	if serverSummary.Mode != "background" {
		t.Errorf("run-server mode = %q, want %q", serverSummary.Mode, "background")
	}

	// Dev frontend step should have terminal mode.
	devSummary := findSummary(result.StepSummaries, "dev-frontend")
	if devSummary == nil {
		t.Fatal("dev-frontend step not found")
	}
	if devSummary.Mode != "terminal" {
		t.Errorf("dev-frontend mode = %q, want %q", devSummary.Mode, "terminal")
	}
}

func findSummary(summaries []steprunner.StepSummary, id string) *steprunner.StepSummary {
	for i := range summaries {
		if summaries[i].ID == id {
			return &summaries[i]
		}
	}
	return nil
}
