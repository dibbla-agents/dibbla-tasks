package output

import (
	"fmt"
	"io"
	"sync"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// GHActionsFormatter emits GitHub Actions workflow commands to an io.Writer.
//
// See: https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions
type GHActionsFormatter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewGHActionsFormatter creates a formatter that writes GitHub Actions
// workflow commands to w.
func NewGHActionsFormatter(w io.Writer) *GHActionsFormatter {
	return &GHActionsFormatter{w: w}
}

func (f *GHActionsFormatter) StartStep(stepID, stepName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::group::%s\n", stepName)
}

func (f *GHActionsFormatter) EndStep(stepID, stepName string, success bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(f.w, "::endgroup::")
}

func (f *GHActionsFormatter) SkipStep(stepID, stepName, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::warning title=Skipped::%s — %s\n", stepName, reason)
}

func (f *GHActionsFormatter) Error(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::error title=%s::%s\n", title, message)
}

func (f *GHActionsFormatter) Warning(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::warning title=%s::%s\n", title, message)
}

func (f *GHActionsFormatter) Notice(title, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::notice title=%s::%s\n", title, message)
}

func (f *GHActionsFormatter) Debug(message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintf(f.w, "::debug::%s\n", message)
}

func (f *GHActionsFormatter) Stdout(stepID, line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(f.w, line)
}

func (f *GHActionsFormatter) Stderr(stepID, line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Fprintln(f.w, line)
}

var _ model.Formatter = (*GHActionsFormatter)(nil)
