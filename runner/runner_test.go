package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/output"
	"github.com/dibbla-agents/dibbla-tasks/runner"
	"github.com/dibbla-agents/dibbla-tasks/toolcheck"
)

func newTestRunner(buf *bytes.Buffer) *runner.Runner {
	return runner.New(runner.Config{
		Executor:  executor.NewShellExecutor(),
		Checker:   toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor()),
		Formatter: output.NewGHActionsFormatter(buf),
	})
}

func TestRunner_SimpleCommand(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "echo",
				Name: "Echo Hello",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "echo hello_from_runner"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != model.StepStatusSuccess {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusSuccess)
	}

	out := buf.String()
	if !strings.Contains(out, "::group::Echo Hello") {
		t.Error("missing group start in output")
	}
	if !strings.Contains(out, "hello_from_runner") {
		t.Error("missing echo output")
	}
	if !strings.Contains(out, "::endgroup::") {
		t.Error("missing group end in output")
	}
}

func TestRunner_FailingCommand(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "fail",
				Name: "Failing Step",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "exit 42"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if result.Steps[0].Status != model.StepStatusFailed {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusFailed)
	}
	if result.Steps[0].ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.Steps[0].ExitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "::error") {
		t.Error("missing error annotation in output")
	}
}

func TestRunner_DependencyOrder(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:        "second",
				Name:      "Second",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo SECOND"},
				DependsOn: []string{"first"},
			},
			{
				ID:   "first",
				Name: "First",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "echo FIRST"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].StepID != "first" {
		t.Errorf("first executed step = %q, want %q", result.Steps[0].StepID, "first")
	}
	if result.Steps[1].StepID != "second" {
		t.Errorf("second executed step = %q, want %q", result.Steps[1].StepID, "second")
	}
}

func TestRunner_SkipOnFailedDependency(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "fail",
				Name: "Failing",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "exit 1"},
			},
			{
				ID:        "after-fail",
				Name:      "After Fail",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo should_not_run"},
				DependsOn: []string{"fail"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if result.Steps[1].Status != model.StepStatusSkipped {
		t.Errorf("step status = %q, want %q", result.Steps[1].Status, model.StepStatusSkipped)
	}
	if strings.Contains(buf.String(), "should_not_run") {
		t.Error("skipped step should not have produced output")
	}
}

func TestRunner_SkipPropagatesTransitively(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "fail",
				Name: "Failing",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "exit 1"},
			},
			{
				ID:        "middle",
				Name:      "Middle",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo middle_should_not_run"},
				DependsOn: []string{"fail"},
			},
			{
				ID:        "end",
				Name:      "End",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo end_should_not_run"},
				DependsOn: []string{"middle"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(result.Steps))
	}
	if result.Steps[1].Status != model.StepStatusSkipped {
		t.Errorf("middle step status = %q, want %q", result.Steps[1].Status, model.StepStatusSkipped)
	}
	if result.Steps[2].Status != model.StepStatusSkipped {
		t.Errorf("end step status = %q, want %q", result.Steps[2].Status, model.StepStatusSkipped)
	}

	out := buf.String()
	if strings.Contains(out, "middle_should_not_run") {
		t.Error("middle step should not have produced output")
	}
	if strings.Contains(out, "end_should_not_run") {
		t.Error("end step should not have produced output")
	}
}

func TestRunner_ContinueOnError(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:              "soft-fail",
				Name:            "Soft Fail",
				Type:            model.StepTypeCommand,
				Run:             model.PlatformString{Common: "exit 1"},
				ContinueOnError: true,
			},
			{
				ID:        "after-soft",
				Name:      "After Soft Fail",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo continued"},
				DependsOn: []string{"soft-fail"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps[0].Status != model.StepStatusFailed {
		t.Errorf("first step status = %q, want %q", result.Steps[0].Status, model.StepStatusFailed)
	}
	if result.Steps[1].Status != model.StepStatusSuccess {
		t.Errorf("second step status = %q, want %q", result.Steps[1].Status, model.StepStatusSuccess)
	}
	if !strings.Contains(buf.String(), "continued") {
		t.Error("second step should have run")
	}
}

func TestRunner_ToolCheck_ExistingTool(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Tools: map[string]model.ToolDef{
			"git": {Check: model.PlatformString{Common: "git --version"}},
		},
		Steps: []model.Step{
			{
				ID:   "check-git",
				Name: "Check Git",
				Type: model.StepTypeToolCheck,
				Tool: "git",
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success — git should be available")
	}
	if result.Steps[0].Status != model.StepStatusSuccess {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusSuccess)
	}

	out := buf.String()
	if !strings.Contains(out, "::notice title=Tool Check") {
		t.Error("missing tool check notice in output")
	}
}

func TestRunner_ToolCheck_MissingTool_NoInstall(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Tools: map[string]model.ToolDef{
			"nonexistent_xyz": {Check: model.PlatformString{Common: "nonexistent_xyz --version"}},
		},
		Steps: []model.Step{
			{
				ID:   "check-missing",
				Name: "Check Missing Tool",
				Type: model.StepTypeToolCheck,
				Tool: "nonexistent_xyz",
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing tool")
	}
	if result.Steps[0].Status != model.StepStatusFailed {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusFailed)
	}

	out := buf.String()
	if !strings.Contains(out, "::error") {
		t.Error("missing error annotation in output")
	}
}

func TestRunner_EnvMerge(t *testing.T) {
	var buf bytes.Buffer
	r := runner.New(runner.Config{
		Executor:  executor.NewShellExecutor(),
		Checker:   toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor()),
		Formatter: output.NewGHActionsFormatter(&buf),
		Env:       map[string]string{"BASE_VAR": "base_value"},
	})

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "env-test",
				Name: "Env Test",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{PerPlatform: map[string]string{
					"darwin":  "echo $BASE_VAR $STEP_VAR",
					"linux":   "echo $BASE_VAR $STEP_VAR",
					"windows": "echo %BASE_VAR% %STEP_VAR%",
				}},
				Env:  map[string]string{"STEP_VAR": "step_value"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	if !strings.Contains(out, "base_value") {
		t.Error("missing base env var in output")
	}
	if !strings.Contains(out, "step_value") {
		t.Error("missing step env var in output")
	}
}

// DAG tests

func TestTopologicalSort_SimpleChain(t *testing.T) {
	steps := []model.Step{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "a"},
	}
	order, err := runner.TopologicalSort(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"a", "b", "c"}
	if len(order) != len(expected) {
		t.Fatalf("order length = %d, want %d", len(order), len(expected))
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want)
		}
	}
}

func TestTopologicalSort_Diamond(t *testing.T) {
	steps := []model.Step{
		{ID: "d", DependsOn: []string{"b", "c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "a"},
	}
	order, err := runner.TopologicalSort(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("order length = %d, want 4", len(order))
	}
	if order[0] != "a" {
		t.Errorf("first = %q, want %q", order[0], "a")
	}
	if order[3] != "d" {
		t.Errorf("last = %q, want %q", order[3], "d")
	}
}

func TestTopologicalSort_NoDeps(t *testing.T) {
	steps := []model.Step{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}
	order, err := runner.TopologicalSort(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("order length = %d, want 3", len(order))
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	steps := []model.Step{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	_, err := runner.TopologicalSort(steps)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if !strings.Contains(err.Error(), "cyclic") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestTopologicalSort_SelfCycle(t *testing.T) {
	steps := []model.Step{
		{ID: "a", DependsOn: []string{"a"}},
	}
	_, err := runner.TopologicalSort(steps)
	if err == nil {
		t.Fatal("expected error for self-cycle")
	}
}

// --- Environment Variable and write_env Tests ---

func newTestRunnerWithDir(buf *bytes.Buffer, workDir string) *runner.Runner {
	return runner.New(runner.Config{
		Executor:  executor.NewShellExecutor(),
		Checker:   toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor()),
		Formatter: output.NewGHActionsFormatter(buf),
		WorkDir:   workDir,
	})
}

func newTestRunnerWithEnv(buf *bytes.Buffer, workDir string, env map[string]string) *runner.Runner {
	return runner.New(runner.Config{
		Executor:  executor.NewShellExecutor(),
		Checker:   toolcheck.NewChecker(toolcheck.RuntimePlatform{}, executor.NewShellExecutor()),
		Formatter: output.NewGHActionsFormatter(buf),
		WorkDir:   workDir,
		Env:       env,
	})
}

func TestRunner_WriteEnvStep(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	r := newTestRunnerWithEnv(&buf, dir, map[string]string{
		"MY_VAR": "my_value",
	})

	sf := &model.StepFile{
		Version: "1",
		Env: map[string]model.EnvVarDef{
			"MY_VAR": {Description: "A test variable", Default: "default_val"},
		},
		Steps: []model.Step{
			{
				ID:   "write-env",
				Name: "Write Env",
				Type: model.StepTypeWriteEnv,
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	// Verify the .env file was created.
	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "MY_VAR=my_value") {
		t.Errorf("expected MY_VAR=my_value in .env, got: %s", content)
	}
	if !strings.Contains(content, "# A test variable") {
		t.Errorf("expected description comment in .env, got: %s", content)
	}
}

func TestRunner_WriteEnvCustomPath(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	r := newTestRunnerWithEnv(&buf, dir, map[string]string{
		"KEY": "value",
	})

	sf := &model.StepFile{
		Version: "1",
		Env: map[string]model.EnvVarDef{
			"KEY": {Default: "default"},
		},
		Steps: []model.Step{
			{
				ID:      "write-env",
				Name:    "Write Env",
				Type:    model.StepTypeWriteEnv,
				EnvFile: ".env.local",
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	if _, err := os.Stat(filepath.Join(dir, ".env.local")); os.IsNotExist(err) {
		t.Error("expected .env.local to exist")
	}
}

func TestRunner_EnvVarsFlowToSteps(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	r := newTestRunnerWithDir(&buf, dir)

	sf := &model.StepFile{
		Version: "1",
		Env: map[string]model.EnvVarDef{
			"PIPELINE_VAR": {Default: "from_pipeline"},
		},
		Steps: []model.Step{
			{
				ID:   "echo-env",
				Name: "Echo Env",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{PerPlatform: map[string]string{
					"darwin":  "echo $PIPELINE_VAR",
					"linux":   "echo $PIPELINE_VAR",
					"windows": "echo %PIPELINE_VAR%",
				}},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	if !strings.Contains(out, "from_pipeline") {
		t.Errorf("expected pipeline env var in output, got: %s", out)
	}
}

func TestRunner_RequiredEnvMissing(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunner(&buf)

	sf := &model.StepFile{
		Version: "1",
		Env: map[string]model.EnvVarDef{
			"REQUIRED_KEY": {Required: true, Description: "Must be set"},
		},
		Steps: []model.Step{
			{
				ID:   "test",
				Name: "Test",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "echo test"},
			},
		},
	}

	_, err := r.Run(context.Background(), sf)
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	if !errors.Is(err, model.ErrMissingRequiredEnv) {
		t.Errorf("error = %v, want ErrMissingRequiredEnv", err)
	}
}

func TestRunner_WriteEnvWithDefaults(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	r := newTestRunnerWithDir(&buf, dir)

	sf := &model.StepFile{
		Version: "1",
		Env: map[string]model.EnvVarDef{
			"APP_NAME": {Description: "App name", Default: "MyApp"},
			"APP_PORT": {Description: "Port", Default: "3000"},
		},
		Steps: []model.Step{
			{
				ID:   "write-env",
				Name: "Write Env",
				Type: model.StepTypeWriteEnv,
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "APP_NAME=MyApp") {
		t.Error("missing APP_NAME=MyApp")
	}
	if !strings.Contains(content, "APP_PORT=3000") {
		t.Error("missing APP_PORT=3000")
	}
}

// --- Platform Filtering Tests ---

type fakePlatform struct{ os string }

func (f fakePlatform) OS() string { return f.os }

func newTestRunnerWithPlatform(buf *bytes.Buffer, platform string) *runner.Runner {
	fp := fakePlatform{os: platform}
	return runner.New(runner.Config{
		Executor:  executor.NewShellExecutor(),
		Checker:   toolcheck.NewChecker(fp, executor.NewShellExecutor()),
		Formatter: output.NewGHActionsFormatter(buf),
		Platform:  fp,
	})
}

func TestFilterStepsForPlatform_KeepsAllWhenNoPlatforms(t *testing.T) {
	steps := []model.Step{
		{ID: "a"},
		{ID: "b"},
	}
	filtered := runner.FilterStepsForPlatform(steps, "darwin")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(filtered))
	}
}

func TestFilterStepsForPlatform_FiltersNonMatching(t *testing.T) {
	steps := []model.Step{
		{ID: "a", Platforms: []string{"darwin"}},
		{ID: "b", Platforms: []string{"linux"}},
		{ID: "c"},
	}
	filtered := runner.FilterStepsForPlatform(steps, "darwin")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(filtered))
	}
	if filtered[0].ID != "a" || filtered[1].ID != "c" {
		t.Errorf("got [%s, %s], want [a, c]", filtered[0].ID, filtered[1].ID)
	}
}

func TestFilterStepsForPlatform_PrunesDependencies(t *testing.T) {
	steps := []model.Step{
		{ID: "linux-step", Platforms: []string{"linux"}},
		{ID: "after", DependsOn: []string{"linux-step", "common"}},
		{ID: "common"},
	}
	filtered := runner.FilterStepsForPlatform(steps, "darwin")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(filtered))
	}
	afterStep := filtered[0] // "after"
	if afterStep.ID != "after" {
		t.Fatalf("expected 'after', got %q", afterStep.ID)
	}
	if len(afterStep.DependsOn) != 1 || afterStep.DependsOn[0] != "common" {
		t.Errorf("DependsOn = %v, want [common]", afterStep.DependsOn)
	}
}

func TestRunner_PlatformFilterSkipsNonApplicableSteps(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunnerWithPlatform(&buf, "darwin")

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:        "linux-only",
				Name:      "Linux Only",
				Type:      model.StepTypeCommand,
				Platforms: []string{"linux"},
				Run:       model.PlatformString{Common: "echo should_not_run"},
			},
			{
				ID:   "common",
				Name: "Common Step",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "echo common_output"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	// Only the common step should be in results.
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(result.Steps))
	}
	if result.Steps[0].StepID != "common" {
		t.Errorf("step id = %q, want %q", result.Steps[0].StepID, "common")
	}

	out := buf.String()
	if strings.Contains(out, "should_not_run") {
		t.Error("linux-only step should not have run on darwin")
	}
	if !strings.Contains(out, "common_output") {
		t.Error("common step should have run")
	}
}

func TestRunner_PlatformSpecificRunCommand(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunnerWithPlatform(&buf, "darwin")

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "install",
				Name: "Install",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{
					PerPlatform: map[string]string{
						"darwin":  "echo darwin_install",
						"linux":   "echo linux_install",
						"windows": "echo windows_install",
					},
				},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}

	out := buf.String()
	if !strings.Contains(out, "darwin_install") {
		t.Error("expected darwin-specific command output")
	}
	if strings.Contains(out, "linux_install") {
		t.Error("should not contain linux command output")
	}
}

func TestRunner_PlatformRunMapMissingPlatform(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunnerWithPlatform(&buf, "darwin")

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "no-darwin",
				Name: "No Darwin",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{
					PerPlatform: map[string]string{
						"linux": "echo linux_only",
					},
				},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when no run command for platform")
	}
	if result.Steps[0].Status != model.StepStatusFailed {
		t.Errorf("status = %q, want %q", result.Steps[0].Status, model.StepStatusFailed)
	}
}

func TestRunner_RetrySucceedsOnSecondAttempt(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "attempt")

	var buf bytes.Buffer
	r := newTestRunnerWithDir(&buf, dir)

	// Script that fails with "EPERM" on first run, succeeds on second.
	// Uses a marker file to track attempts.
	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "retry-step",
				Name: "Retry Step",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{PerPlatform: map[string]string{
					"darwin":  fmt.Sprintf("if [ -f '%s' ]; then echo OK; else touch '%s' && echo 'EPERM: operation not permitted' >&2 && exit 1; fi", marker, marker),
					"linux":   fmt.Sprintf("if [ -f '%s' ]; then echo OK; else touch '%s' && echo 'EPERM: operation not permitted' >&2 && exit 1; fi", marker, marker),
					"windows": fmt.Sprintf("if exist \"%s\" (echo OK) else (echo. > \"%s\" & echo EPERM: operation not permitted >&2 & exit /b 1)", marker, marker),
				}},
				Retry: &model.RetryPolicy{
					MaxAttempts: 2,
					DelayStr:    "100ms",
					OnOutput:    "EPERM",
				},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success after retry, got failure: %s", result.Steps[0].Error)
	}
	if result.Steps[0].Status != model.StepStatusSuccess {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusSuccess)
	}

	out := buf.String()
	if !strings.Contains(out, "Retry") {
		t.Error("expected retry warning in output")
	}
}

func TestRunner_RetryNotTriggeredWhenOutputDoesNotMatch(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "no-retry-marker")

	var buf bytes.Buffer
	r := newTestRunnerWithDir(&buf, dir)

	// This command always fails with "TIMEOUT" (not "EPERM"), so retry
	// should not be triggered despite max_attempts=2.
	// We use a marker file to detect if a second attempt happened.
	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:   "no-retry",
				Name: "No Retry",
				Type: model.StepTypeCommand,
				Run: model.PlatformString{PerPlatform: map[string]string{
					"darwin":  fmt.Sprintf("touch '%s' && echo TIMEOUT && exit 1", marker),
					"linux":   fmt.Sprintf("touch '%s' && echo TIMEOUT && exit 1", marker),
					"windows": fmt.Sprintf("echo. > \"%s\" & echo TIMEOUT & exit /b 1", marker),
				}},
				Retry: &model.RetryPolicy{
					MaxAttempts: 2,
					DelayStr:    "100ms",
					OnOutput:    "EPERM",
				},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure — output doesn't match retry condition")
	}
	if result.Steps[0].Status != model.StepStatusFailed {
		t.Errorf("step status = %q, want %q", result.Steps[0].Status, model.StepStatusFailed)
	}

	out := buf.String()
	if strings.Contains(out, "Retrying step") {
		t.Errorf("should not have retried — output doesn't contain EPERM.\nFull output:\n%s", out)
	}
}

func TestRunner_PlatformFilterWithDependencyPruning(t *testing.T) {
	var buf bytes.Buffer
	r := newTestRunnerWithPlatform(&buf, "darwin")

	sf := &model.StepFile{
		Version: "1",
		Steps: []model.Step{
			{
				ID:        "linux-dep",
				Name:      "Linux Dep",
				Type:      model.StepTypeCommand,
				Platforms: []string{"linux"},
				Run:       model.PlatformString{Common: "echo linux"},
			},
			{
				ID:   "darwin-dep",
				Name: "Darwin Dep",
				Type: model.StepTypeCommand,
				Run:  model.PlatformString{Common: "echo darwin_output"},
			},
			{
				ID:        "final",
				Name:      "Final",
				Type:      model.StepTypeCommand,
				Run:       model.PlatformString{Common: "echo final_output"},
				DependsOn: []string{"linux-dep", "darwin-dep"},
			},
		},
	}

	result, err := r.Run(context.Background(), sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	// linux-dep filtered out, darwin-dep and final should run
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].StepID != "darwin-dep" {
		t.Errorf("first step = %q, want %q", result.Steps[0].StepID, "darwin-dep")
	}
	if result.Steps[1].StepID != "final" {
		t.Errorf("second step = %q, want %q", result.Steps[1].StepID, "final")
	}

	out := buf.String()
	if !strings.Contains(out, "final_output") {
		t.Error("final step should have run despite linux-dep being filtered")
	}
}
