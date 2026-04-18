package steprunner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/output"
	"github.com/dibbla-agents/dibbla-tasks/parser"
	"github.com/dibbla-agents/dibbla-tasks/runner"
	"github.com/dibbla-agents/dibbla-tasks/toolcheck"
)

// Run parses a dibbla-task.yaml file at filePath and executes all steps.
// The working directory defaults to the directory containing the task file
// unless overridden by WithWorkDir.
func Run(ctx context.Context, filePath string, opts ...Option) (*model.RunResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task file: %w", err)
	}
	defer f.Close()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve task file path: %w", err)
	}
	fileDir := filepath.Dir(absPath)

	// Prepend a default WithWorkDir so callers can still override it.
	opts = append([]Option{WithWorkDir(fileDir)}, opts...)

	return RunReader(ctx, f, opts...)
}

// RunReader parses a dibbla-task.yaml from an io.Reader and executes all steps.
func RunReader(ctx context.Context, r io.Reader, opts ...Option) (*model.RunResult, error) {
	cfg := &runConfig{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	p := parser.NewYAMLParser()
	sf, err := p.Parse(r)
	if err != nil {
		return nil, err
	}

	formatter := cfg.formatter
	if formatter == nil {
		formatter = output.NewGHActionsFormatter(cfg.stdout)
	}

	var ex executor.Executor
	if cfg.exec != nil {
		ex = cfg.exec
	} else {
		ex = executor.NewShellExecutor()
	}
	platform := toolcheck.RuntimePlatform{}
	checker := toolcheck.NewChecker(platform, ex)

	rn := runner.New(runner.Config{
		Executor:    ex,
		Checker:     checker,
		Formatter:   formatter,
		Platform:    platform,
		WorkDir:     cfg.workDir,
		Env:         cfg.env,
		KillTracked: cfg.killTracked,
	})

	return rn.Run(ctx, sf)
}

// Preview parses a dibbla-task.yaml file and returns a structured
// description of all steps without executing anything.
func Preview(filePath string) (*model.PreviewResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task file: %w", err)
	}
	defer f.Close()

	return PreviewReader(f)
}

// PreviewReader parses a dibbla-task.yaml from an io.Reader and returns
// a structured preview without execution.
func PreviewReader(r io.Reader) (*model.PreviewResult, error) {
	p := parser.NewYAMLParser()
	sf, err := p.Parse(r)
	if err != nil {
		return nil, err
	}

	order, err := runner.TopologicalSort(sf.Steps)
	if err != nil {
		return nil, err
	}

	summaries := make([]model.StepSummary, len(sf.Steps))
	stepIdx := make(map[string]int, len(sf.Steps))
	for i, step := range sf.Steps {
		stepIdx[step.ID] = i
		summaries[i] = model.StepSummary{
			ID:        step.ID,
			Name:      step.Name,
			Type:      step.Type,
			Mode:      step.Mode,
			DependsOn: step.DependsOn,
			Tool:      step.Tool,
			Platforms: step.Platforms,
		}
	}

	orderedSummaries := make([]model.StepSummary, len(order))
	for i, id := range order {
		orderedSummaries[i] = summaries[stepIdx[id]]
	}

	// Build sorted env var summaries for frontend rendering.
	var envVars []model.EnvVarSummary
	for name, def := range sf.Env {
		envVars = append(envVars, model.EnvVarSummary{
			Name:        name,
			Description: def.Description,
			Default:     def.Default,
			Required:    def.Required,
		})
	}
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})

	// Build sorted port summaries for frontend rendering.
	var ports []model.PortSummary
	for name, def := range sf.Ports {
		ports = append(ports, model.PortSummary{
			Name:        name,
			Preferred:   def.Preferred,
			Description: def.Description,
		})
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name < ports[j].Name
	})

	return &model.PreviewResult{
		File:           sf,
		ExecutionOrder: order,
		StepSummaries:  orderedSummaries,
		EnvVars:        envVars,
		Ports:          ports,
	}, nil
}

// PreviewString is a convenience that parses YAML from a string.
func PreviewString(yamlContent string) (*model.PreviewResult, error) {
	return PreviewReader(strings.NewReader(yamlContent))
}

// RunString is a convenience that runs steps from a YAML string.
func RunString(ctx context.Context, yamlContent string, opts ...Option) (*model.RunResult, error) {
	return RunReader(ctx, strings.NewReader(yamlContent), opts...)
}
