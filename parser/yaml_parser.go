package parser

import (
	"errors"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/dibbla-agents/dibbla-tasks/model"
)

// envVarNameRe matches valid environment variable names.
var envVarNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var supportedVersions = map[string]bool{
	"1": true,
}

// YAMLParser parses dibbla-task.yaml files from YAML sources.
type YAMLParser struct{}

// NewYAMLParser returns a new YAMLParser.
func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

// Parse reads YAML from r and returns a validated StepFile.
func (p *YAMLParser) Parse(r io.Reader) (*model.StepFile, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrParseFailed, err)
	}

	var sf model.StepFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrParseFailed, err)
	}

	if err := validate(&sf); err != nil {
		return nil, err
	}

	return &sf, nil
}

func validate(sf *model.StepFile) error {
	if !supportedVersions[sf.Version] {
		return fmt.Errorf("%w: got %q", model.ErrInvalidVersion, sf.Version)
	}

	if len(sf.Steps) == 0 {
		return model.ErrNoSteps
	}

	// Validate environment variable names.
	for name := range sf.Env {
		if !envVarNameRe.MatchString(name) {
			return fmt.Errorf("%w: %q", model.ErrInvalidEnvVarName, name)
		}
	}

	// Validate port declarations.
	for name, pd := range sf.Ports {
		if !envVarNameRe.MatchString(name) {
			return fmt.Errorf("%w: %q", model.ErrInvalidPortName, name)
		}
		if pd.Preferred < 1024 || pd.Preferred > 65535 {
			return fmt.Errorf("%w: port %q has preferred %d", model.ErrInvalidPortValue, name, pd.Preferred)
		}
		if _, exists := sf.Env[name]; exists {
			return fmt.Errorf("%w: %q is declared in both ports and env", model.ErrPortEnvCollision, name)
		}
	}

	ids := make(map[string]bool, len(sf.Steps))

	for i, step := range sf.Steps {
		if step.ID == "" {
			return fmt.Errorf("%w (step index %d)", model.ErrMissingStepID, i)
		}
		if step.Name == "" {
			return fmt.Errorf("%w (step %q)", model.ErrMissingStepName, step.ID)
		}
		if step.Type == "" {
			return fmt.Errorf("%w (step %q)", model.ErrMissingStepType, step.ID)
		}

		// Validate platforms field.
		for _, p := range step.Platforms {
			if !model.ValidPlatforms[p] {
				return fmt.Errorf("%w: %q in step %q", model.ErrInvalidPlatform, p, step.ID)
			}
		}

		// Validate per-platform run map keys.
		for key := range step.Run.PerPlatform {
			if !model.ValidPlatforms[key] {
				return fmt.Errorf("%w: run map key %q in step %q", model.ErrInvalidPlatform, key, step.ID)
			}
		}

		// Validate mode field.
		if step.Mode != "" {
			if !model.ValidStepModes[step.Mode] {
				return fmt.Errorf("%w: got %q (step %q)", model.ErrInvalidStepMode, step.Mode, step.ID)
			}
			if step.Type != model.StepTypeCommand {
				return fmt.Errorf("%w (step %q has type %q)", model.ErrModeOnNonCommand, step.ID, step.Type)
			}
		}

		switch step.Type {
		case model.StepTypeCommand:
			if step.Run.IsEmpty() {
				return fmt.Errorf("%w (step %q)", model.ErrMissingRunField, step.ID)
			}
		case model.StepTypeToolCheck:
			if step.Tool == "" {
				return fmt.Errorf("%w (step %q)", model.ErrMissingToolField, step.ID)
			}
			if sf.Tools == nil {
				return fmt.Errorf("%w: step %q references tool %q but no tools are defined",
					model.ErrToolNotDefined, step.ID, step.Tool)
			}
			if _, ok := sf.Tools[step.Tool]; !ok {
				return fmt.Errorf("%w: step %q references undefined tool %q",
					model.ErrToolNotDefined, step.ID, step.Tool)
			}
		case model.StepTypeWriteEnv:
			// write_env steps require no additional fields (env_file is optional).
		default:
			return fmt.Errorf("%w: got %q (step %q)", model.ErrInvalidStepType, step.Type, step.ID)
		}

		if ids[step.ID] {
			return fmt.Errorf("%w: %q", model.ErrDuplicateStepID, step.ID)
		}
		ids[step.ID] = true
	}

	for _, step := range sf.Steps {
		for _, dep := range step.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("%w: step %q depends on %q",
					model.ErrDependencyNotFound, step.ID, dep)
			}
		}
	}

	return nil
}

var _ Parser = (*YAMLParser)(nil)

// IsValidationError returns true if the error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, model.ErrInvalidVersion) ||
		errors.Is(err, model.ErrMissingStepID) ||
		errors.Is(err, model.ErrMissingStepName) ||
		errors.Is(err, model.ErrMissingStepType) ||
		errors.Is(err, model.ErrInvalidStepType) ||
		errors.Is(err, model.ErrMissingRunField) ||
		errors.Is(err, model.ErrMissingToolField) ||
		errors.Is(err, model.ErrToolNotDefined) ||
		errors.Is(err, model.ErrDuplicateStepID) ||
		errors.Is(err, model.ErrDependencyNotFound) ||
		errors.Is(err, model.ErrNoSteps) ||
		errors.Is(err, model.ErrInvalidEnvVarName) ||
		errors.Is(err, model.ErrInvalidPlatform) ||
		errors.Is(err, model.ErrInvalidStepMode) ||
		errors.Is(err, model.ErrModeOnNonCommand) ||
		errors.Is(err, model.ErrInvalidPortName) ||
		errors.Is(err, model.ErrInvalidPortValue) ||
		errors.Is(err, model.ErrPortEnvCollision)
}
