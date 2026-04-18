package steprunner

import "github.com/dibbla-agents/dibbla-tasks/model"

// Error variables re-exported from the model package for public API consumers.

var (
	ErrParseFailed        = model.ErrParseFailed
	ErrInvalidVersion     = model.ErrInvalidVersion
	ErrMissingStepID      = model.ErrMissingStepID
	ErrMissingStepName    = model.ErrMissingStepName
	ErrMissingStepType    = model.ErrMissingStepType
	ErrInvalidStepType    = model.ErrInvalidStepType
	ErrMissingRunField    = model.ErrMissingRunField
	ErrMissingToolField   = model.ErrMissingToolField
	ErrToolNotDefined     = model.ErrToolNotDefined
	ErrDuplicateStepID    = model.ErrDuplicateStepID
	ErrDependencyNotFound = model.ErrDependencyNotFound
	ErrCyclicDependency   = model.ErrCyclicDependency
	ErrStepFailed         = model.ErrStepFailed
	ErrToolNotFound       = model.ErrToolNotFound
	ErrToolInstallFailed  = model.ErrToolInstallFailed
	ErrNoSteps            = model.ErrNoSteps
	ErrMissingRequiredEnv = model.ErrMissingRequiredEnv
	ErrInvalidEnvVarName  = model.ErrInvalidEnvVarName
	ErrInvalidPlatform    = model.ErrInvalidPlatform
	ErrNoPlatformCommand  = model.ErrNoPlatformCommand
)
