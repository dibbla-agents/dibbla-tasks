// Package runner orchestrates the execution of steps from a parsed StepFile.
package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dibbla-agents/dibbla-tasks/executor"
	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/netcheck"
	"github.com/dibbla-agents/dibbla-tasks/portalloc"
	"github.com/dibbla-agents/dibbla-tasks/toolcheck"
	"github.com/dibbla-agents/dibbla-tasks/xcodewatch"
)

// Runner orchestrates step execution.
type Runner struct {
	exec        executor.Executor
	checker     *toolcheck.Checker
	formatter   model.Formatter
	platform    toolcheck.Platform
	workDir     string
	env         map[string]string
	portEnv     map[string]string // dynamically allocated ports — always win over step env
	killTracked bool
}

// Config holds the runner configuration.
type Config struct {
	Executor    executor.Executor
	Checker     *toolcheck.Checker
	Formatter   model.Formatter
	Platform    toolcheck.Platform
	WorkDir     string
	Env         map[string]string
	KillTracked bool
}

// New creates a Runner with the given configuration.
func New(cfg Config) *Runner {
	p := cfg.Platform
	if p == nil {
		p = toolcheck.RuntimePlatform{}
	}
	return &Runner{
		exec:        cfg.Executor,
		checker:     cfg.Checker,
		formatter:   cfg.Formatter,
		platform:    p,
		workDir:     cfg.WorkDir,
		env:         cfg.Env,
		killTracked: cfg.KillTracked,
	}
}

// resolveDir returns the effective working directory for a step.
// If stepDir is empty, the runner's workDir is used.
// If stepDir is absolute, it is used as-is.
// If stepDir is relative, it is joined to the runner's workDir.
func (r *Runner) resolveDir(stepDir string) string {
	if stepDir == "" {
		return r.workDir
	}
	if filepath.IsAbs(stepDir) {
		return stepDir
	}
	return filepath.Join(r.workDir, stepDir)
}

// Run executes all steps in the StepFile in dependency order.
func (r *Runner) Run(ctx context.Context, sf *model.StepFile) (*model.RunResult, error) {
	startTime := time.Now()

	// Optionally kill background processes still running from a previous run.
	// Only fires when the caller passes WithKillTracked() (e.g. re-installs).
	// "Open With" runs deliberately skip this so they don't kill servers
	// started by a prior install.
	if r.killTracked {
		if kt, ok := r.exec.(interface{ KillTracked() }); ok {
			kt.KillTracked()
		}
	}

	// Resolve pipeline-level environment variables before any step runs.
	if len(sf.Env) > 0 {
		resolved, err := ResolveEnv(sf.Env, r.env)
		if err != nil {
			return nil, err
		}
		r.env = resolved
	}

	// Allocate dynamic ports. These are stored separately from r.env so they
	// can be applied last during per-step env merging, overriding any hardcoded
	// PORT values in step-level env (backward-compat fallbacks for old runners).
	if len(sf.Ports) > 0 {
		defs := make(map[string]portalloc.PortDef, len(sf.Ports))
		for name, pd := range sf.Ports {
			defs[name] = portalloc.PortDef{
				Preferred:   pd.Preferred,
				Description: pd.Description,
				Range:       pd.Range,
			}
		}
		allocated, err := portalloc.AllocateAll(defs)
		if err != nil {
			return nil, err
		}
		r.portEnv = make(map[string]string, len(allocated))
		for name, port := range allocated {
			r.portEnv[name] = strconv.Itoa(port)
		}
	}

	// Filter steps for the current platform before sorting.
	applicable := FilterStepsForPlatform(sf.Steps, r.platform.OS())

	order, err := TopologicalSort(applicable)
	if err != nil {
		return nil, err
	}

	stepMap := make(map[string]*model.Step, len(applicable))
	for i := range applicable {
		stepMap[applicable[i].ID] = &applicable[i]
	}

	result := &model.RunResult{
		Success: true,
	}

	failed := make(map[string]bool)

	// Detect background Xcode CLT installs on macOS and emit notices.
	stopXcodeWatch := xcodewatch.Watch(ctx, r.formatter)
	defer stopXcodeWatch()

	for _, id := range order {
		step := stepMap[id]

		shouldSkip := false
		for _, dep := range step.DependsOn {
			if failed[dep] {
				shouldSkip = true
				break
			}
		}

		if shouldSkip {
			sr := model.StepResult{
				StepID:   step.ID,
				StepName: step.Name,
				Status:   model.StepStatusSkipped,
				ExitCode: -1,
			}
			result.Steps = append(result.Steps, sr)
			failed[step.ID] = true

			failedDep := ""
			for _, dep := range step.DependsOn {
				if failed[dep] {
					failedDep = stepMap[dep].Name
					break
				}
			}
			r.formatter.SkipStep(step.ID, step.Name,
				fmt.Sprintf("Depends on failed step: %s", failedDep))
			continue
		}

		sr := r.executeStep(ctx, step, sf)
		result.Steps = append(result.Steps, sr)

		if sr.Status == model.StepStatusFailed && !step.ContinueOnError {
			failed[step.ID] = true
			result.Success = false
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

func (r *Runner) executeStep(ctx context.Context, step *model.Step, sf *model.StepFile) model.StepResult {
	startTime := time.Now()
	r.formatter.StartStep(step.ID, step.Name)

	// On Windows, a previous step may have installed software that updated
	// the registry PATH (e.g. winget, npm install -g). Refresh the process
	// PATH before every step so the latest binaries are always visible.
	if err := toolcheck.RefreshSystemPath(); err != nil {
		r.formatter.Debug(fmt.Sprintf("PATH refresh: %v (continuing anyway)", err))
	}

	var sr model.StepResult
	sr.StepID = step.ID
	sr.StepName = step.Name

	switch step.Type {
	case model.StepTypeToolCheck:
		sr = r.executeToolCheck(ctx, step, sf)
	case model.StepTypeCommand:
		sr = r.executeCommand(ctx, step)
	case model.StepTypeWriteEnv:
		sr = r.executeWriteEnv(step, sf)
	default:
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("unknown step type: %s", step.Type)
		r.formatter.Error("Unknown Step Type", sr.Error)
	}

	sr.Duration = time.Since(startTime)
	success := sr.Status == model.StepStatusSuccess
	r.formatter.EndStep(step.ID, step.Name, success)

	return sr
}

func (r *Runner) executeToolCheck(ctx context.Context, step *model.Step, sf *model.StepFile) model.StepResult {
	sr := model.StepResult{
		StepID:   step.ID,
		StepName: step.Name,
	}

	toolDef, ok := sf.Tools[step.Tool]
	if !ok {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("tool %q not defined in tools map", step.Tool)
		r.formatter.Error("Tool Not Defined", sr.Error)
		return sr
	}

	checkCmd, _ := toolDef.Check.Resolve(r.platform.OS())
	r.formatter.Debug(fmt.Sprintf("Checking if '%s' is available...", step.Tool))

	checkResult, err := r.checker.IsAvailable(ctx, step.Tool, checkCmd)
	if err != nil {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("failed to check tool %q: %v", step.Tool, err)
		r.formatter.Error("Tool Check Error", sr.Error)
		return sr
	}

	if checkResult.Available {
		msg := fmt.Sprintf("%s is installed", step.Tool)
		if checkResult.Path != "" {
			msg += fmt.Sprintf(" at %s", checkResult.Path)
		}
		if output := strings.TrimSpace(checkResult.Output); output != "" {
			r.formatter.Stdout(step.ID, output)
		}
		r.formatter.Notice("Tool Check", msg)
		sr.Status = model.StepStatusSuccess
		sr.ExitCode = 0
		return sr
	}

	r.formatter.Warning("Tool Missing", fmt.Sprintf("%s not found, attempting install...", step.Tool))

	if len(toolDef.Install) == 0 && len(toolDef.InstallFallback) == 0 {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("tool %q not found and no install commands defined", step.Tool)
		r.formatter.Error("Tool Not Found", sr.Error)
		return sr
	}

	stdoutWriter := &formatterWriter{formatter: r.formatter, stepID: step.ID, isStderr: false}
	stderrWriter := &formatterWriter{formatter: r.formatter, stepID: step.ID, isStderr: true}
	installOpts := toolcheck.InstallOpts{
		Stdout: stdoutWriter,
		Stderr: stderrWriter,
	}

	// Try primary install first.
	primaryFailed := false
	if len(toolDef.Install) > 0 {
		err = r.checker.Install(ctx, step.Tool, toolDef, installOpts)
		if err != nil {
			primaryFailed = true
			if len(toolDef.InstallFallback) > 0 {
				r.formatter.Warning("Install Failed", fmt.Sprintf("primary install of %q failed, trying fallback...", step.Tool))
			} else {
				sr.Status = model.StepStatusFailed
				sr.ExitCode = -1
				sr.Error = fmt.Sprintf("failed to install %q: %v", step.Tool, err)
				r.formatter.Error("Install Failed", sr.Error)
				return sr
			}
		}
	}

	// Try fallback install if primary failed or was not defined.
	if primaryFailed || len(toolDef.Install) == 0 {
		if len(toolDef.InstallFallback) > 0 {
			err = r.checker.InstallFallback(ctx, step.Tool, toolDef, installOpts)
			if err != nil {
				sr.Status = model.StepStatusFailed
				sr.ExitCode = -1
				sr.Error = fmt.Sprintf("fallback install of %q failed: %v", step.Tool, err)
				r.formatter.Error("Fallback Install Failed", sr.Error)
				return sr
			}
		}
	}

	// If bin_path is set, prepend it to the process PATH and persist
	// it to ~/.dibbla/env so external processes (e.g. Claude Code
	// launched via "Open With") can also find the tool.
	if toolDef.BinPath != "" {
		expanded, err := toolcheck.ExpandBinPath(toolDef.BinPath)
		if err != nil {
			r.formatter.Debug(fmt.Sprintf("bin_path expand: %v (continuing anyway)", err))
		} else {
			if err := toolcheck.PrependBinPath(expanded); err != nil {
				r.formatter.Debug(fmt.Sprintf("bin_path prepend: %v (continuing anyway)", err))
			}
			if err := toolcheck.PersistBinPath(expanded); err != nil {
				r.formatter.Debug(fmt.Sprintf("bin_path persist: %v (continuing anyway)", err))
			}
			r.formatter.Debug(fmt.Sprintf("Added %s to PATH", toolDef.BinPath))
		}
	}

	// Refresh PATH — the install may have added new entries to the
	// Windows registry (e.g. winget) that the current process doesn't see.
	if err := toolcheck.RefreshSystemPath(); err != nil {
		r.formatter.Debug(fmt.Sprintf("PATH refresh: %v (continuing anyway)", err))
	}

	checkResult, _ = r.checker.IsAvailable(ctx, step.Tool, checkCmd)
	if checkResult != nil && checkResult.Available {
		r.formatter.Notice("Tool Installed", fmt.Sprintf("%s successfully installed", step.Tool))
		sr.Status = model.StepStatusSuccess
		sr.ExitCode = 0
	} else {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("tool %q still not available after install", step.Tool)
		r.formatter.Error("Install Verification Failed", sr.Error)
	}

	return sr
}

func (r *Runner) executeCommand(ctx context.Context, step *model.Step) model.StepResult {
	sr := model.StepResult{
		StepID:   step.ID,
		StepName: step.Name,
	}

	runCmd, ok := step.Run.Resolve(r.platform.OS())
	if !ok {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("%v: step %q has no run command for %q",
			model.ErrNoPlatformCommand, step.ID, r.platform.OS())
		r.formatter.Error("No Platform Command", sr.Error)
		return sr
	}

	env := make(map[string]string)
	for k, v := range r.env {
		env[k] = v
	}
	for k, v := range step.Env {
		env[k] = v
	}
	// Port-allocated env vars always win, so templates can keep hardcoded
	// env values (e.g. PORT: "8080") as fallbacks for older runners that
	// don't understand the ports: section.
	for k, v := range r.portEnv {
		env[k] = v
	}

	dir := r.resolveDir(step.WorkingDir)

	// Map the step mode to executor mode.
	var execMode executor.ExecMode
	switch step.Mode {
	case model.StepModeBackground:
		execMode = executor.ExecModeBackground
	case model.StepModeTerminal:
		execMode = executor.ExecModeTerminal
	}

	// Retry configuration.
	maxAttempts := 1
	var retryDelay time.Duration
	var retryOnOutput string
	if step.Retry != nil && step.Retry.MaxAttempts > 1 {
		maxAttempts = step.Retry.MaxAttempts
		retryDelay = step.Retry.GetDelay()
		retryOnOutput = strings.ToLower(step.Retry.OnOutput)
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			r.formatter.Warning("Retry",
				fmt.Sprintf("Retrying step %q (attempt %d/%d) after %s...",
					step.Name, attempt, maxAttempts, retryDelay))
			select {
			case <-ctx.Done():
				sr.Status = model.StepStatusFailed
				sr.ExitCode = -1
				sr.Error = "cancelled during retry wait"
				return sr
			case <-time.After(retryDelay):
			}
		}

		r.formatter.Debug(fmt.Sprintf("Running: %s", runCmd))

		// Tee output to the formatter (for UI) and a buffer (for retry matching).
		var outputBuf strings.Builder
		stdoutWriter := io.MultiWriter(
			&formatterWriter{formatter: r.formatter, stepID: step.ID, isStderr: false},
			&outputBuf,
		)
		stderrWriter := io.MultiWriter(
			&formatterWriter{formatter: r.formatter, stepID: step.ID, isStderr: true},
			&outputBuf,
		)

		result, err := r.exec.Execute(ctx, runCmd, executor.ExecOpts{
			Env:        env,
			Dir:        dir,
			Stdout:     stdoutWriter,
			Stderr:     stderrWriter,
			Mode:       execMode,
			StepName:   step.Name,
			ProjectDir: r.workDir,
			Port:       inferPort(step.Env, r.portEnv),
		})

		if err != nil {
			// Hard exec errors (e.g. binary not found) are not retryable.
			sr.Status = model.StepStatusFailed
			sr.ExitCode = -1
			sr.Error = fmt.Sprintf("failed to execute: %v", err)
			r.formatter.Error("Execution Error", sr.Error)
			return sr
		}

		if result.ExitCode == 0 {
			sr.Status = model.StepStatusSuccess
			sr.ExitCode = 0
			return sr
		}

		// Command failed — check if we should retry.
		shouldRetry := attempt < maxAttempts
		if shouldRetry && retryOnOutput != "" {
			shouldRetry = strings.Contains(strings.ToLower(outputBuf.String()), retryOnOutput)
		}

		if shouldRetry {
			continue
		}

		// Final failure.
		sr.ExitCode = result.ExitCode
		sr.Status = model.StepStatusFailed
		sr.Error = fmt.Sprintf("command exited with code %d", result.ExitCode)

		// Check if the network is down — this explains many install failures.
		if offline, msg := netcheck.IsOffline(ctx); offline {
			sr.NetworkError = true
			sr.Error = msg
			r.formatter.Warning("Network Error", msg)
		} else if strings.Contains(strings.ToLower(outputBuf.String()), "eperm") {
			// If EPERM appeared in output, hint about antivirus.
			sr.Error += ". This may be caused by antivirus software (e.g. Windows Defender) " +
				"locking files during installation. Try excluding your development folder " +
				"from real-time scanning"
			r.formatter.Warning("Antivirus Hint",
				"EPERM errors can be caused by antivirus real-time scanning. "+
					"Consider excluding your development folder from Windows Defender.")
		}

		r.formatter.Error("Step Failed",
			fmt.Sprintf("Step %q failed with exit code %d", step.Name, result.ExitCode))
		return sr
	}

	return sr
}

func (r *Runner) executeWriteEnv(step *model.Step, sf *model.StepFile) model.StepResult {
	sr := model.StepResult{
		StepID:   step.ID,
		StepName: step.Name,
	}

	filename := step.EnvFile
	if filename == "" {
		filename = ".env"
	}

	r.formatter.Debug(fmt.Sprintf("Writing environment variables to %s", filename))

	// Only write declared env vars (from sf.Env), resolved through r.env.
	vars := make(map[string]string)
	for name := range sf.Env {
		if val, ok := r.env[name]; ok {
			vars[name] = val
		}
	}

	dir := r.resolveDir(step.WorkingDir)

	if err := WriteEnvFile(dir, filename, vars, sf.Env); err != nil {
		sr.Status = model.StepStatusFailed
		sr.ExitCode = -1
		sr.Error = fmt.Sprintf("failed to write env file: %v", err)
		r.formatter.Error("Write Env Failed", sr.Error)
		return sr
	}

	r.formatter.Notice("Write Env", fmt.Sprintf("Wrote %d variables to %s", len(vars), filename))
	sr.Status = model.StepStatusSuccess
	sr.ExitCode = 0
	return sr
}

// ansiPattern matches ANSI CSI sequences (e.g. \x1b[2K, \x1b[1;31m)
// and OSC sequences (e.g. \x1b]0;title\x07).
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\][^\x07]*\x07`)

// stripANSI removes all ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// formatterWriter routes each line of output through a Formatter.
// It strips ANSI escape sequences and handles \r (carriage return) so
// that spinner animations collapse to a single line instead of producing
// one row per frame.
type formatterWriter struct {
	formatter model.Formatter
	stepID    string
	isStderr  bool
}

func (w *formatterWriter) Write(p []byte) (int, error) {
	text := stripANSI(string(p))

	// Split on newlines first.
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// Strip trailing \r left over from Windows \r\n line endings
		// before applying the spinner collapse logic.
		line = strings.TrimRight(line, "\r")

		// Handle carriage returns: keep only the text after the last \r.
		// This collapses spinner frames (which use \r to overwrite the line)
		// into a single output line showing the final state.
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if w.isStderr {
			w.formatter.Stderr(w.stepID, line)
		} else {
			w.formatter.Stdout(w.stepID, line)
		}
	}

	return len(p), nil
}

var _ io.Writer = (*formatterWriter)(nil)

// inferPort returns the most likely port for a background step.
// It first checks the step's own env (what the YAML step declares),
// then falls back to pipeline-level allocated ports. This prevents
// every background step from showing the same global PORT value.
func inferPort(stepEnv map[string]string, portEnv map[string]string) string {
	portKeys := []string{"PORT", "VITE_PORT", "DEV_PORT", "SERVER_PORT"}

	// Prefer port variables explicitly set on the step.
	for _, key := range portKeys {
		if v, ok := stepEnv[key]; ok && v != "" {
			// If the ports section allocated a different value, use that
			// (it overrides the step-level hardcoded fallback).
			if allocated, ok := portEnv[key]; ok {
				return allocated
			}
			return v
		}
	}

	// No step-level port — check if the step's working directory or
	// name hints at which allocated port to use. Fall back to showing
	// nothing rather than showing an unrelated port.
	return ""
}
