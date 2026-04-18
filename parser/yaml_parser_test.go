package parser_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/parser"
)

func TestYAMLParser_ValidBasic(t *testing.T) {
	input := `
version: "1"
steps:
  - id: hello
    name: "Say Hello"
    type: command
    run: "echo hello"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Version != "1" {
		t.Errorf("version = %q, want %q", sf.Version, "1")
	}
	if len(sf.Steps) != 1 {
		t.Fatalf("steps count = %d, want 1", len(sf.Steps))
	}
	s := sf.Steps[0]
	if s.ID != "hello" {
		t.Errorf("step id = %q, want %q", s.ID, "hello")
	}
	if s.Name != "Say Hello" {
		t.Errorf("step name = %q, want %q", s.Name, "Say Hello")
	}
	if s.Type != model.StepTypeCommand {
		t.Errorf("step type = %q, want %q", s.Type, model.StepTypeCommand)
	}
	if s.Run.Common != "echo hello" {
		t.Errorf("step run = %q, want %q", s.Run.Common, "echo hello")
	}
}

func TestYAMLParser_ValidWithTools(t *testing.T) {
	input := `
version: "1"
tools:
  go:
    check: "go version"
    install:
      darwin: "brew install go"
      linux: "sudo apt-get install -y golang"
steps:
  - id: check-go
    name: "Check Go"
    type: tool_check
    tool: go
  - id: build
    name: "Build"
    type: command
    run: "go build ./..."
    depends_on: ["check-go"]
    env:
      CGO_ENABLED: "0"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.Tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(sf.Tools))
	}
	tool, ok := sf.Tools["go"]
	if !ok {
		t.Fatal("tool 'go' not found")
	}
	if tool.Check.Common != "go version" {
		t.Errorf("tool check = %q, want %q", tool.Check.Common, "go version")
	}
	if tool.Install["darwin"] != "brew install go" {
		t.Errorf("tool install darwin = %q, want %q", tool.Install["darwin"], "brew install go")
	}
	if len(sf.Steps) != 2 {
		t.Fatalf("steps count = %d, want 2", len(sf.Steps))
	}
	if len(sf.Steps[1].DependsOn) != 1 || sf.Steps[1].DependsOn[0] != "check-go" {
		t.Errorf("step depends_on = %v, want [check-go]", sf.Steps[1].DependsOn)
	}
	if sf.Steps[1].Env["CGO_ENABLED"] != "0" {
		t.Errorf("step env CGO_ENABLED = %q, want %q", sf.Steps[1].Env["CGO_ENABLED"], "0")
	}
}

func TestYAMLParser_InvalidVersion(t *testing.T) {
	input := `
version: "99"
steps:
  - id: x
    name: "X"
    type: command
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
	if !errors.Is(err, model.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestYAMLParser_MissingVersion(t *testing.T) {
	input := `
steps:
  - id: x
    name: "X"
    type: command
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !errors.Is(err, model.ErrInvalidVersion) {
		t.Errorf("error = %v, want ErrInvalidVersion", err)
	}
}

func TestYAMLParser_MissingStepID(t *testing.T) {
	input := `
version: "1"
steps:
  - name: "No ID"
    type: command
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrMissingStepID) {
		t.Errorf("error = %v, want ErrMissingStepID", err)
	}
}

func TestYAMLParser_MissingStepName(t *testing.T) {
	input := `
version: "1"
steps:
  - id: no-name
    type: command
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrMissingStepName) {
		t.Errorf("error = %v, want ErrMissingStepName", err)
	}
}

func TestYAMLParser_InvalidStepType(t *testing.T) {
	input := `
version: "1"
steps:
  - id: bad
    name: "Bad"
    type: unknown_type
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrInvalidStepType) {
		t.Errorf("error = %v, want ErrInvalidStepType", err)
	}
}

func TestYAMLParser_CommandMissingRun(t *testing.T) {
	input := `
version: "1"
steps:
  - id: noruncmd
    name: "No Run"
    type: command
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrMissingRunField) {
		t.Errorf("error = %v, want ErrMissingRunField", err)
	}
}

func TestYAMLParser_ToolCheckMissingTool(t *testing.T) {
	input := `
version: "1"
tools:
  go:
    check: "go version"
steps:
  - id: check
    name: "Check"
    type: tool_check
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrMissingToolField) {
		t.Errorf("error = %v, want ErrMissingToolField", err)
	}
}

func TestYAMLParser_ToolNotDefined(t *testing.T) {
	input := `
version: "1"
steps:
  - id: check
    name: "Check"
    type: tool_check
    tool: nonexistent
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrToolNotDefined) {
		t.Errorf("error = %v, want ErrToolNotDefined", err)
	}
}

func TestYAMLParser_DuplicateStepID(t *testing.T) {
	input := `
version: "1"
steps:
  - id: same
    name: "First"
    type: command
    run: "echo 1"
  - id: same
    name: "Second"
    type: command
    run: "echo 2"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrDuplicateStepID) {
		t.Errorf("error = %v, want ErrDuplicateStepID", err)
	}
}

func TestYAMLParser_DependencyNotFound(t *testing.T) {
	input := `
version: "1"
steps:
  - id: build
    name: "Build"
    type: command
    run: "echo build"
    depends_on: ["nonexistent"]
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrDependencyNotFound) {
		t.Errorf("error = %v, want ErrDependencyNotFound", err)
	}
}

func TestYAMLParser_NoSteps(t *testing.T) {
	input := `
version: "1"
steps: []
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if !errors.Is(err, model.ErrNoSteps) {
		t.Errorf("error = %v, want ErrNoSteps", err)
	}
}

func TestYAMLParser_InvalidYAMLSyntax(t *testing.T) {
	input := `
version: "1"
steps:
  - id: [broken yaml
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !errors.Is(err, model.ErrParseFailed) {
		t.Errorf("error = %v, want ErrParseFailed", err)
	}
}

func TestYAMLParser_ContinueOnError(t *testing.T) {
	input := `
version: "1"
steps:
  - id: fragile
    name: "Fragile"
    type: command
    run: "echo might fail"
    continue_on_error: true
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sf.Steps[0].ContinueOnError {
		t.Error("continue_on_error should be true")
	}
}

// --- Environment Variable Tests ---

func TestYAMLParser_ValidWithEnvVars(t *testing.T) {
	input := `
version: "1"
env:
  DATABASE_URL:
    description: "PostgreSQL connection string"
    default: "postgres://localhost:5432/mydb"
  API_KEY:
    description: "Third-party API key"
    required: true
  LOG_LEVEL:
    description: "Logging verbosity"
    default: "info"
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
  - id: build
    name: "Build"
    type: command
    run: "echo build"
    depends_on: ["write-env"]
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.Env) != 3 {
		t.Fatalf("env count = %d, want 3", len(sf.Env))
	}

	dbDef := sf.Env["DATABASE_URL"]
	if dbDef.Description != "PostgreSQL connection string" {
		t.Errorf("DATABASE_URL description = %q", dbDef.Description)
	}
	if dbDef.Default != "postgres://localhost:5432/mydb" {
		t.Errorf("DATABASE_URL default = %q", dbDef.Default)
	}

	apiDef := sf.Env["API_KEY"]
	if !apiDef.Required {
		t.Error("API_KEY should be required")
	}
	if apiDef.Description != "Third-party API key" {
		t.Errorf("API_KEY description = %q", apiDef.Description)
	}
}

func TestYAMLParser_WriteEnvStep(t *testing.T) {
	input := `
version: "1"
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
    env_file: ".env.local"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].Type != model.StepTypeWriteEnv {
		t.Errorf("step type = %q, want %q", sf.Steps[0].Type, model.StepTypeWriteEnv)
	}
	if sf.Steps[0].EnvFile != ".env.local" {
		t.Errorf("env_file = %q, want %q", sf.Steps[0].EnvFile, ".env.local")
	}
}

func TestYAMLParser_WriteEnvNoEnvFile(t *testing.T) {
	input := `
version: "1"
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].EnvFile != "" {
		t.Errorf("env_file should be empty, got %q", sf.Steps[0].EnvFile)
	}
}

func TestYAMLParser_InvalidEnvVarName(t *testing.T) {
	input := `
version: "1"
env:
  2FAST:
    description: "Starts with a digit"
    default: "bad"
steps:
  - id: test
    name: "Test"
    type: command
    run: "echo test"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid env var name")
	}
	if !errors.Is(err, model.ErrInvalidEnvVarName) {
		t.Errorf("error = %v, want ErrInvalidEnvVarName", err)
	}
}

func TestYAMLParser_InvalidEnvVarNameWithDash(t *testing.T) {
	input := `
version: "1"
env:
  MY-VAR:
    description: "Has a dash"
    default: "bad"
steps:
  - id: test
    name: "Test"
    type: command
    run: "echo test"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for env var name with dash")
	}
	if !errors.Is(err, model.ErrInvalidEnvVarName) {
		t.Errorf("error = %v, want ErrInvalidEnvVarName", err)
	}
}

func TestYAMLParser_EnvVarNoStepFields(t *testing.T) {
	// write_env step should not require 'run' or 'tool' fields
	input := `
version: "1"
env:
  MY_VAR:
    default: "hello"
steps:
  - id: write-env
    name: "Write Env"
    type: write_env
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Platform Support Tests ---

func TestYAMLParser_RunAsString(t *testing.T) {
	input := `
version: "1"
steps:
  - id: build
    name: "Build"
    type: command
    run: "echo hello"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := sf.Steps[0]
	if s.Run.Common != "echo hello" {
		t.Errorf("Run.Common = %q, want %q", s.Run.Common, "echo hello")
	}
	if len(s.Run.PerPlatform) != 0 {
		t.Errorf("Run.PerPlatform should be empty, got %v", s.Run.PerPlatform)
	}
}

func TestYAMLParser_RunAsPlatformMap(t *testing.T) {
	input := `
version: "1"
steps:
  - id: install
    name: "Install"
    type: command
    run:
      darwin: "brew install foo"
      linux: "apt install foo"
      windows: "choco install foo"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := sf.Steps[0]
	if s.Run.Common != "" {
		t.Errorf("Run.Common should be empty, got %q", s.Run.Common)
	}
	if len(s.Run.PerPlatform) != 3 {
		t.Fatalf("Run.PerPlatform should have 3 entries, got %d", len(s.Run.PerPlatform))
	}
	if s.Run.PerPlatform["darwin"] != "brew install foo" {
		t.Errorf("Run.PerPlatform[darwin] = %q", s.Run.PerPlatform["darwin"])
	}
	if s.Run.PerPlatform["linux"] != "apt install foo" {
		t.Errorf("Run.PerPlatform[linux] = %q", s.Run.PerPlatform["linux"])
	}
	if s.Run.PerPlatform["windows"] != "choco install foo" {
		t.Errorf("Run.PerPlatform[windows] = %q", s.Run.PerPlatform["windows"])
	}
}

func TestYAMLParser_RunAsPartialPlatformMap(t *testing.T) {
	input := `
version: "1"
steps:
  - id: install
    name: "Install"
    type: command
    platforms: ["darwin"]
    run:
      darwin: "brew install foo"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := sf.Steps[0]
	if len(s.Run.PerPlatform) != 1 {
		t.Fatalf("expected 1 platform entry, got %d", len(s.Run.PerPlatform))
	}
}

func TestYAMLParser_PlatformsField(t *testing.T) {
	input := `
version: "1"
steps:
  - id: mac-step
    name: "Mac Step"
    type: command
    platforms: ["darwin", "linux"]
    run: "echo hello"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := sf.Steps[0]
	if len(s.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(s.Platforms))
	}
	if s.Platforms[0] != "darwin" || s.Platforms[1] != "linux" {
		t.Errorf("platforms = %v, want [darwin linux]", s.Platforms)
	}
}

func TestYAMLParser_InvalidPlatformInPlatformsField(t *testing.T) {
	input := `
version: "1"
steps:
  - id: bad
    name: "Bad"
    type: command
    platforms: ["darwin", "bsd"]
    run: "echo hello"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid platform name")
	}
	if !errors.Is(err, model.ErrInvalidPlatform) {
		t.Errorf("error = %v, want ErrInvalidPlatform", err)
	}
}

func TestYAMLParser_InvalidPlatformInRunMap(t *testing.T) {
	input := `
version: "1"
steps:
  - id: bad
    name: "Bad"
    type: command
    run:
      darwin: "echo hello"
      freebsd: "echo hello"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid platform key in run map")
	}
	if !errors.Is(err, model.ErrInvalidPlatform) {
		t.Errorf("error = %v, want ErrInvalidPlatform", err)
	}
}

// --- Privileged Tool Tests ---

func TestYAMLParser_ToolWithPrivileged(t *testing.T) {
	input := `
version: "1"
tools:
  brew:
    check: "brew --version"
    privileged: true
    install:
      darwin: "echo install_brew"
steps:
  - id: check-brew
    name: "Check Brew"
    type: tool_check
    tool: brew
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tool, ok := sf.Tools["brew"]
	if !ok {
		t.Fatal("tool 'brew' not found")
	}
	if !tool.Privileged {
		t.Error("expected brew tool to have privileged = true")
	}
	if tool.Check.Common != "brew --version" {
		t.Errorf("tool check = %q, want %q", tool.Check.Common, "brew --version")
	}
	if tool.Install["darwin"] != "echo install_brew" {
		t.Errorf("tool install darwin = %q, want %q", tool.Install["darwin"], "echo install_brew")
	}
}

func TestYAMLParser_ToolWithoutPrivileged(t *testing.T) {
	input := `
version: "1"
tools:
  go:
    check: "go version"
    install:
      darwin: "brew install go"
steps:
  - id: check-go
    name: "Check Go"
    type: tool_check
    tool: go
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tool, ok := sf.Tools["go"]
	if !ok {
		t.Fatal("tool 'go' not found")
	}
	if tool.Privileged {
		t.Error("expected go tool to have privileged = false by default")
	}
}

// --- Step Mode Tests ---

func TestYAMLParser_ModeBackground(t *testing.T) {
	input := `
version: "1"
steps:
  - id: server
    name: "Start Server"
    type: command
    mode: background
    run: "./server"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].Mode != model.StepModeBackground {
		t.Errorf("mode = %q, want %q", sf.Steps[0].Mode, model.StepModeBackground)
	}
}

func TestYAMLParser_ModeTerminal(t *testing.T) {
	input := `
version: "1"
steps:
  - id: dev
    name: "Dev Server"
    type: command
    mode: terminal
    run: "npm run dev"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].Mode != model.StepModeTerminal {
		t.Errorf("mode = %q, want %q", sf.Steps[0].Mode, model.StepModeTerminal)
	}
}

func TestYAMLParser_ModeDefaultIsEmpty(t *testing.T) {
	input := `
version: "1"
steps:
  - id: build
    name: "Build"
    type: command
    run: "echo build"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].Mode != "" {
		t.Errorf("mode = %q, want empty string (foreground default)", sf.Steps[0].Mode)
	}
}

func TestYAMLParser_InvalidMode(t *testing.T) {
	input := `
version: "1"
steps:
  - id: bad
    name: "Bad"
    type: command
    mode: "daemon"
    run: "echo x"
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !errors.Is(err, model.ErrInvalidStepMode) {
		t.Errorf("error = %v, want ErrInvalidStepMode", err)
	}
}

func TestYAMLParser_ModeOnNonCommandStep(t *testing.T) {
	input := `
version: "1"
tools:
  go:
    check: "go version"
steps:
  - id: check
    name: "Check Go"
    type: tool_check
    tool: go
    mode: background
`
	p := parser.NewYAMLParser()
	_, err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for mode on non-command step")
	}
	if !errors.Is(err, model.ErrModeOnNonCommand) {
		t.Errorf("error = %v, want ErrModeOnNonCommand", err)
	}
}

func TestYAMLParser_RetryPolicy(t *testing.T) {
	input := `
version: "1"
steps:
  - id: install
    name: "Install"
    type: command
    run: "npm ci"
    retry:
      max_attempts: 2
      delay: "5s"
      on_output: "EPERM"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := sf.Steps[0]
	if s.Retry == nil {
		t.Fatal("expected retry policy to be set")
	}
	if s.Retry.MaxAttempts != 2 {
		t.Errorf("max_attempts = %d, want 2", s.Retry.MaxAttempts)
	}
	if s.Retry.DelayStr != "5s" {
		t.Errorf("delay = %q, want %q", s.Retry.DelayStr, "5s")
	}
	if s.Retry.OnOutput != "EPERM" {
		t.Errorf("on_output = %q, want %q", s.Retry.OnOutput, "EPERM")
	}
	if s.Retry.GetDelay().Seconds() != 5 {
		t.Errorf("GetDelay() = %v, want 5s", s.Retry.GetDelay())
	}
}

func TestYAMLParser_RetryPolicyAbsent(t *testing.T) {
	input := `
version: "1"
steps:
  - id: build
    name: "Build"
    type: command
    run: "echo build"
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Steps[0].Retry != nil {
		t.Error("expected retry to be nil when not specified")
	}
}

func TestYAMLParser_ToolPrivilegedFalseExplicit(t *testing.T) {
	input := `
version: "1"
tools:
  node:
    check: "node --version"
    privileged: false
    install:
      darwin: "brew install node"
steps:
  - id: check-node
    name: "Check Node"
    type: tool_check
    tool: node
`
	p := parser.NewYAMLParser()
	sf, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tool := sf.Tools["node"]
	if tool.Privileged {
		t.Error("expected node tool to have privileged = false")
	}
}
