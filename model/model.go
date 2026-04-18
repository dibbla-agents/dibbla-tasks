// Package model defines the shared types, interfaces, and errors used
// across the steprunner sub-packages. This package exists to break import
// cycles — all sub-packages import model, and the root steprunner package
// re-exports these types for public API consumers.
package model

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// ValidPlatforms lists the accepted platform identifiers.
var ValidPlatforms = map[string]bool{
	"darwin":  true,
	"linux":   true,
	"windows": true,
}

// PlatformString represents a value that can be either a plain string
// (applying to all platforms) or a per-platform map keyed by OS name.
type PlatformString struct {
	// Common holds the value when run is a plain string (all platforms).
	Common string
	// PerPlatform holds platform-specific values when run is a map.
	PerPlatform map[string]string
}

// Resolve returns the command string for the given platform.
// It returns the common value if set, otherwise looks up the platform key.
// The second return value indicates whether a value was found.
func (ps PlatformString) Resolve(platform string) (string, bool) {
	if ps.Common != "" {
		return ps.Common, true
	}
	if cmd, ok := ps.PerPlatform[platform]; ok {
		return cmd, true
	}
	return "", false
}

// IsEmpty returns true if the PlatformString has no value at all.
func (ps PlatformString) IsEmpty() bool {
	return ps.Common == "" && len(ps.PerPlatform) == 0
}

// UnmarshalYAML implements custom YAML unmarshaling so that `run` can be
// either a scalar string or a mapping of platform names to strings.
func (ps *PlatformString) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		ps.Common = value.Value
		return nil
	case yaml.MappingNode:
		ps.PerPlatform = make(map[string]string)
		return value.Decode(&ps.PerPlatform)
	default:
		return fmt.Errorf("run must be a string or a platform map, got YAML kind %d", value.Kind)
	}
}

// MarshalJSON serialises PlatformString as a JSON string when Common is
// set, or as a JSON object when PerPlatform is set.
func (ps PlatformString) MarshalJSON() ([]byte, error) {
	if ps.Common != "" {
		return json.Marshal(ps.Common)
	}
	if len(ps.PerPlatform) > 0 {
		return json.Marshal(ps.PerPlatform)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON deserialises PlatformString from either a JSON string or
// a JSON object.
func (ps *PlatformString) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		ps.Common = s
		return nil
	}
	// Try map.
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		ps.PerPlatform = m
		return nil
	}
	return fmt.Errorf("run must be a JSON string or object")
}

// StepFile represents a parsed dibbla-task.yaml file.
type StepFile struct {
	Version string               `yaml:"version"`
	Env     map[string]EnvVarDef `yaml:"env,omitempty"`
	Ports   map[string]PortDef   `yaml:"ports,omitempty"`
	Tools   map[string]ToolDef   `yaml:"tools,omitempty"`
	Steps   []Step               `yaml:"steps"`
}

// PortDef declares a named port that is dynamically allocated at runtime.
// The port name becomes an environment variable available to all steps.
type PortDef struct {
	Preferred   int    `yaml:"preferred"              json:"preferred"`
	Description string `yaml:"description,omitempty"  json:"description,omitempty"`
	Range       int    `yaml:"range,omitempty"         json:"range,omitempty"`
}

// EnvVarDef describes a pipeline-level environment variable declared
// in the top-level env section of a dibbla-task.yaml file.
type EnvVarDef struct {
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Default     string `yaml:"default,omitempty"     json:"default,omitempty"`
	Required    bool   `yaml:"required,omitempty"    json:"required,omitempty"`
}

// ToolDef defines how to detect and install a tool.
type ToolDef struct {
	Check           PlatformString    `yaml:"check"`
	Install         map[string]string `yaml:"install,omitempty"`
	InstallFallback map[string]string `yaml:"install_fallback,omitempty"`
	BinPath         string            `yaml:"bin_path,omitempty"`
	Privileged      bool              `yaml:"privileged,omitempty"`
}

// StepType enumerates the kinds of steps supported.
type StepType string

const (
	StepTypeCommand   StepType = "command"
	StepTypeToolCheck StepType = "tool_check"
	StepTypeWriteEnv  StepType = "write_env"
)

// StepMode controls how a command step is executed.
type StepMode string

const (
	// StepModeForeground (default) blocks until the command completes.
	StepModeForeground StepMode = ""

	// StepModeBackground starts the command and moves on without waiting
	// for it to exit. Useful for background services (e.g. a Go backend).
	StepModeBackground StepMode = "background"

	// StepModeTerminal opens the command in a new native terminal window
	// (Terminal.app on macOS, gnome-terminal on Linux, cmd on Windows).
	// Useful for dev servers where the user wants to see live output.
	StepModeTerminal StepMode = "terminal"
)

// ValidStepModes lists the accepted step mode values.
var ValidStepModes = map[StepMode]bool{
	StepModeForeground: true,
	StepModeBackground: true,
	StepModeTerminal:   true,
}

// Step describes a single unit of work in a dibbla-task.yaml file.
type Step struct {
	ID              string            `yaml:"id"                          json:"id"`
	Name            string            `yaml:"name"                        json:"name"`
	Type            StepType          `yaml:"type"                        json:"type"`
	Tool            string            `yaml:"tool,omitempty"              json:"tool,omitempty"`
	Run             PlatformString    `yaml:"run,omitempty"               json:"run,omitempty"`
	Mode            StepMode          `yaml:"mode,omitempty"              json:"mode,omitempty"`
	Platforms       []string          `yaml:"platforms,omitempty"         json:"platforms,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"               json:"env,omitempty"`
	WorkingDir      string            `yaml:"working_dir,omitempty"       json:"working_dir,omitempty"`
	DependsOn       []string          `yaml:"depends_on,omitempty"        json:"depends_on,omitempty"`
	ContinueOnError bool              `yaml:"continue_on_error,omitempty" json:"continue_on_error,omitempty"`
	EnvFile         string            `yaml:"env_file,omitempty"          json:"env_file,omitempty"`
	Retry           *RetryPolicy      `yaml:"retry,omitempty"             json:"retry,omitempty"`
}

// RetryPolicy controls automatic retry for failed steps.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts (1 = no retry, 2 = one retry).
	MaxAttempts int    `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	// DelayStr is the wait time between attempts (e.g. "3s", "500ms").
	DelayStr    string `yaml:"delay,omitempty"        json:"delay,omitempty"`
	// OnOutput limits retries to failures whose combined stdout/stderr
	// contains this substring (case-insensitive). Empty means always retry.
	OnOutput    string `yaml:"on_output,omitempty"    json:"on_output,omitempty"`
}

// GetDelay parses DelayStr into a time.Duration, defaulting to 3s.
func (rp *RetryPolicy) GetDelay() time.Duration {
	if rp == nil || rp.DelayStr == "" {
		return 3 * time.Second
	}
	d, err := time.ParseDuration(rp.DelayStr)
	if err != nil {
		return 3 * time.Second
	}
	return d
}

// StepStatus represents the lifecycle state of a step.
type StepStatus string

const (
	StepStatusPending StepStatus = "pending"
	StepStatusRunning StepStatus = "running"
	StepStatusSuccess StepStatus = "success"
	StepStatusFailed  StepStatus = "failed"
	StepStatusSkipped StepStatus = "skipped"
)

// EventType distinguishes the category of a StepEvent so frontends can
// cleanly route events without fragile field-presence checks.
type EventType string

const (
	EventTypeStatus  EventType = "status"
	EventTypeStdout  EventType = "stdout"
	EventTypeStderr  EventType = "stderr"
	EventTypeError   EventType = "error"
	EventTypeWarning EventType = "warning"
	EventTypeNotice  EventType = "notice"
	EventTypeDebug   EventType = "debug"
)

// StepEvent is a structured event emitted during step execution.
type StepEvent struct {
	Type      EventType  `json:"type"`
	StepID    string     `json:"step_id"`
	StepName  string     `json:"step_name"`
	Status    StepStatus `json:"status,omitempty"`
	Output    string     `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// StepResult captures the outcome of executing a single step.
type StepResult struct {
	StepID       string        `json:"step_id"`
	StepName     string        `json:"step_name"`
	Status       StepStatus    `json:"status"`
	ExitCode     int           `json:"exit_code"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	NetworkError bool          `json:"network_error,omitempty"`
}

// RunResult captures the outcome of an entire pipeline run.
type RunResult struct {
	Steps    []StepResult  `json:"steps"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
}

// PreviewResult describes the pipeline without executing anything.
type PreviewResult struct {
	File           *StepFile       `json:"file"`
	ExecutionOrder []string        `json:"execution_order"`
	StepSummaries  []StepSummary   `json:"step_summaries"`
	EnvVars        []EnvVarSummary `json:"env_vars,omitempty"`
	Ports          []PortSummary   `json:"ports,omitempty"`
}

// PortSummary is a lightweight description of a declared port for preview.
type PortSummary struct {
	Name        string `json:"name"`
	Preferred   int    `json:"preferred"`
	Description string `json:"description,omitempty"`
}

// EnvVarSummary is a lightweight description of a declared environment
// variable for frontend rendering (input forms, setup screens).
type EnvVarSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
}

// StepSummary is a lightweight description of a step for preview purposes.
type StepSummary struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      StepType `json:"type"`
	Mode      StepMode `json:"mode,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
	Tool      string   `json:"tool,omitempty"`
	Platforms []string `json:"platforms,omitempty"`
}

// Formatter defines the interface for emitting structured output during
// step execution.
type Formatter interface {
	StartStep(stepID, stepName string)
	EndStep(stepID, stepName string, success bool)
	SkipStep(stepID, stepName, reason string)
	Error(title, message string)
	Warning(title, message string)
	Notice(title, message string)
	Debug(message string)
	Stdout(stepID, line string)
	Stderr(stepID, line string)
}
