package output

import (
	"fmt"
	"io"
	"sync"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// PlainFormatter writes human-readable plain text output.
type PlainFormatter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewPlainFormatter creates a formatter that writes plain text to w.
func NewPlainFormatter(w io.Writer) *PlainFormatter {
	return &PlainFormatter{w: w}
}

func (f *PlainFormatter) StartStep(stepID, stepName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "=== STEP: %s (%s) ===\n", stepName, stepID)
}

func (f *PlainFormatter) EndStep(stepID, stepName string, success bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := "OK"
	if !success {
		status = "FAILED"
	}
	fmt.Fprintf(f.w, "=== END: %s [%s] ===\n", stepName, status)
}

func (f *PlainFormatter) SkipStep(stepID, stepName, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "=== SKIP: %s — %s ===\n", stepName, reason)
}

func (f *PlainFormatter) Error(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "ERROR [%s]: %s\n", title, message)
}

func (f *PlainFormatter) Warning(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "WARNING [%s]: %s\n", title, message)
}

func (f *PlainFormatter) Notice(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "NOTICE [%s]: %s\n", title, message)
}

func (f *PlainFormatter) Debug(message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "DEBUG: %s\n", message)
}

func (f *PlainFormatter) Stdout(stepID, line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(f.w, line)
}

func (f *PlainFormatter) Stderr(stepID, line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "[stderr] %s\n", line)
}

var _ model.Formatter = (*PlainFormatter)(nil)
