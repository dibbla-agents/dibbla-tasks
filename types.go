package steprunner

import "github.com/dibbla-agents/dibbla-tasks/model"

// Type aliases re-exported from the model package for public API consumers.
// This allows users to import just "steprunner" while the internal packages
// share types via the model sub-package to avoid import cycles.

type StepFile = model.StepFile
type ToolDef = model.ToolDef
type StepType = model.StepType
type Step = model.Step
type StepStatus = model.StepStatus
type EventType = model.EventType
type StepEvent = model.StepEvent
type StepResult = model.StepResult
type RunResult = model.RunResult
type PreviewResult = model.PreviewResult
type StepSummary = model.StepSummary
type EnvVarDef = model.EnvVarDef
type EnvVarSummary = model.EnvVarSummary
type PlatformString = model.PlatformString
type Formatter = model.Formatter

const (
	StepTypeCommand   = model.StepTypeCommand
	StepTypeToolCheck = model.StepTypeToolCheck
	StepTypeWriteEnv  = model.StepTypeWriteEnv

	StepStatusPending = model.StepStatusPending
	StepStatusRunning = model.StepStatusRunning
	StepStatusSuccess = model.StepStatusSuccess
	StepStatusFailed  = model.StepStatusFailed
	StepStatusSkipped = model.StepStatusSkipped

	EventTypeStatus  = model.EventTypeStatus
	EventTypeStdout  = model.EventTypeStdout
	EventTypeStderr  = model.EventTypeStderr
	EventTypeError   = model.EventTypeError
	EventTypeWarning = model.EventTypeWarning
	EventTypeNotice  = model.EventTypeNotice
	EventTypeDebug   = model.EventTypeDebug
)
