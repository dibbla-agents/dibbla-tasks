package runner_test

import (
	"errors"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/runner"
)

func TestResolveEnv_DefaultsApplied(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"LOG_LEVEL": {Default: "info"},
		"APP_PORT":  {Default: "8080"},
	}
	resolved, err := runner.ResolveEnv(declared, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["LOG_LEVEL"] != "info" {
		t.Errorf("LOG_LEVEL = %q, want %q", resolved["LOG_LEVEL"], "info")
	}
	if resolved["APP_PORT"] != "8080" {
		t.Errorf("APP_PORT = %q, want %q", resolved["APP_PORT"], "8080")
	}
}

func TestResolveEnv_ProvidedOverridesDefault(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"LOG_LEVEL": {Default: "info"},
	}
	provided := map[string]string{
		"LOG_LEVEL": "debug",
	}
	resolved, err := runner.ResolveEnv(declared, provided)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["LOG_LEVEL"] != "debug" {
		t.Errorf("LOG_LEVEL = %q, want %q", resolved["LOG_LEVEL"], "debug")
	}
}

func TestResolveEnv_RequiredMissing(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"API_KEY": {Required: true},
	}
	_, err := runner.ResolveEnv(declared, nil)
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	if !errors.Is(err, model.ErrMissingRequiredEnv) {
		t.Errorf("error = %v, want ErrMissingRequiredEnv", err)
	}
}

func TestResolveEnv_RequiredWithDefault(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"API_KEY": {Required: true, Default: "default-key"},
	}
	resolved, err := runner.ResolveEnv(declared, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["API_KEY"] != "default-key" {
		t.Errorf("API_KEY = %q, want %q", resolved["API_KEY"], "default-key")
	}
}

func TestResolveEnv_RequiredProvidedByUser(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"API_KEY": {Required: true},
	}
	provided := map[string]string{
		"API_KEY": "user-supplied-key",
	}
	resolved, err := runner.ResolveEnv(declared, provided)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["API_KEY"] != "user-supplied-key" {
		t.Errorf("API_KEY = %q, want %q", resolved["API_KEY"], "user-supplied-key")
	}
}

func TestResolveEnv_EmptyDeclarations(t *testing.T) {
	resolved, err := runner.ResolveEnv(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 0 {
		t.Errorf("expected empty resolved map, got %d entries", len(resolved))
	}
}

func TestResolveEnv_ExtraProvidedPassThrough(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"DECLARED_VAR": {Default: "declared"},
	}
	provided := map[string]string{
		"EXTRA_VAR": "extra_value",
	}
	resolved, err := runner.ResolveEnv(declared, provided)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["DECLARED_VAR"] != "declared" {
		t.Errorf("DECLARED_VAR = %q, want %q", resolved["DECLARED_VAR"], "declared")
	}
	if resolved["EXTRA_VAR"] != "extra_value" {
		t.Errorf("EXTRA_VAR = %q, want %q", resolved["EXTRA_VAR"], "extra_value")
	}
}

func TestResolveEnv_MultipleRequiredMissing(t *testing.T) {
	declared := map[string]model.EnvVarDef{
		"KEY_A": {Required: true},
		"KEY_B": {Required: true},
		"KEY_C": {Default: "has-default"},
	}
	_, err := runner.ResolveEnv(declared, nil)
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	if !errors.Is(err, model.ErrMissingRequiredEnv) {
		t.Errorf("error = %v, want ErrMissingRequiredEnv", err)
	}
	// Error message should mention both missing keys.
	errMsg := err.Error()
	if !(contains(errMsg, "KEY_A") && contains(errMsg, "KEY_B")) {
		t.Errorf("error message should mention both KEY_A and KEY_B, got: %s", errMsg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
