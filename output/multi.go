package output

import "github.com/dibbla-agents/dibbla-tasks/model"

// MultiFormatter composes multiple Formatter implementations so events
// are dispatched to all of them.
type MultiFormatter struct {
	formatters []model.Formatter
}

// NewMultiFormatter creates a formatter that delegates to all provided formatters.
func NewMultiFormatter(formatters ...model.Formatter) *MultiFormatter {
	return &MultiFormatter{formatters: formatters}
}

func (m *MultiFormatter) StartStep(stepID, stepName string) {
	for _, f := range m.formatters {
		f.StartStep(stepID, stepName)
	}
}

func (m *MultiFormatter) EndStep(stepID, stepName string, success bool) {
	for _, f := range m.formatters {
		f.EndStep(stepID, stepName, success)
	}
}

func (m *MultiFormatter) SkipStep(stepID, stepName, reason string) {
	for _, f := range m.formatters {
		f.SkipStep(stepID, stepName, reason)
	}
}

func (m *MultiFormatter) Error(title, message string) {
	for _, f := range m.formatters {
		f.Error(title, message)
	}
}

func (m *MultiFormatter) Warning(title, message string) {
	for _, f := range m.formatters {
		f.Warning(title, message)
	}
}

func (m *MultiFormatter) Notice(title, message string) {
	for _, f := range m.formatters {
		f.Notice(title, message)
	}
}

func (m *MultiFormatter) Debug(message string) {
	for _, f := range m.formatters {
		f.Debug(message)
	}
}

func (m *MultiFormatter) Stdout(stepID, line string) {
	for _, f := range m.formatters {
		f.Stdout(stepID, line)
	}
}

func (m *MultiFormatter) Stderr(stepID, line string) {
	for _, f := range m.formatters {
		f.Stderr(stepID, line)
	}
}

var _ model.Formatter = (*MultiFormatter)(nil)
