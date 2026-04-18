// Package parser provides interfaces and implementations for reading
// dibbla-task.yaml files into the steprunner type system.
package parser

import (
	"io"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// Parser reads a dibbla-task.yaml source and produces a StepFile.
type Parser interface {
	Parse(r io.Reader) (*model.StepFile, error)
}
