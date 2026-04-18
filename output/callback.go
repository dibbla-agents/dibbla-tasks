package output

import (
	"time"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// CallbackFormatter emits StepEvent structs via a callback function.
// This is designed for GUI integration — the Wails app layer can bridge
// these events to the frontend via runtime.EventsEmit.
type CallbackFormatter struct {
	onEvent func(model.StepEvent)
}

// NewCallbackFormatter creates a formatter that calls fn for each event.
func NewCallbackFormatter(fn func(model.StepEvent)) *CallbackFormatter {
	return &CallbackFormatter{onEvent: fn}
}

func (f *CallbackFormatter) emit(event model.StepEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	f.onEvent(event)
}

func (f *CallbackFormatter) StartStep(stepID, stepName string) {
	f.emit(model.StepEvent{
		Type:     model.EventTypeStatus,
		StepID:   stepID,
		StepName: stepName,
		Status:   model.StepStatusRunning,
	})
}

func (f *CallbackFormatter) EndStep(stepID, stepName string, success bool) {
	status := model.StepStatusSuccess
	if !success {
		status = model.StepStatusFailed
	}
	f.emit(model.StepEvent{
		Type:     model.EventTypeStatus,
		StepID:   stepID,
		StepName: stepName,
		Status:   status,
	})
}

func (f *CallbackFormatter) SkipStep(stepID, stepName, reason string) {
	f.emit(model.StepEvent{
		Type:     model.EventTypeStatus,
		StepID:   stepID,
		StepName: stepName,
		Status:   model.StepStatusSkipped,
		Output:   reason,
	})
}

func (f *CallbackFormatter) Error(title, message string) {
	f.emit(model.StepEvent{
		Type:  model.EventTypeError,
		Error: title + ": " + message,
	})
}

func (f *CallbackFormatter) Warning(title, message string) {
	f.emit(model.StepEvent{
		Type:   model.EventTypeWarning,
		Output: "WARNING: " + title + ": " + message,
	})
}

func (f *CallbackFormatter) Notice(title, message string) {
	f.emit(model.StepEvent{
		Type:   model.EventTypeNotice,
		Output: "NOTICE: " + title + ": " + message,
	})
}

func (f *CallbackFormatter) Debug(message string) {
	f.emit(model.StepEvent{
		Type:   model.EventTypeDebug,
		Output: "DEBUG: " + message,
	})
}

func (f *CallbackFormatter) Stdout(stepID, line string) {
	f.emit(model.StepEvent{
		Type:   model.EventTypeStdout,
		StepID: stepID,
		Output: line,
	})
}

func (f *CallbackFormatter) Stderr(stepID, line string) {
	f.emit(model.StepEvent{
		Type:   model.EventTypeStderr,
		StepID: stepID,
		Output: line,
	})
}

var _ model.Formatter = (*CallbackFormatter)(nil)
