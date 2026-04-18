package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/runner"
)

func TestWriteEnvFile_BasicOutput(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{
		"APP_PORT":  "8080",
		"LOG_LEVEL": "info",
	}
	defs := map[string]model.EnvVarDef{}

	err := runner.WriteEnvFile(dir, ".env", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "APP_PORT=8080") {
		t.Error("missing APP_PORT=8080")
	}
	if !strings.Contains(content, "LOG_LEVEL=info") {
		t.Error("missing LOG_LEVEL=info")
	}
}

func TestWriteEnvFile_WithDescriptions(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{
		"DATABASE_URL": "postgres://localhost:5432/mydb",
		"LOG_LEVEL":    "debug",
	}
	defs := map[string]model.EnvVarDef{
		"DATABASE_URL": {Description: "PostgreSQL connection string"},
		"LOG_LEVEL":    {Description: "Logging verbosity"},
	}

	err := runner.WriteEnvFile(dir, ".env", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# PostgreSQL connection string") {
		t.Error("missing description comment for DATABASE_URL")
	}
	if !strings.Contains(content, "# Logging verbosity") {
		t.Error("missing description comment for LOG_LEVEL")
	}
	if !strings.Contains(content, "DATABASE_URL=postgres://localhost:5432/mydb") {
		t.Error("missing DATABASE_URL value")
	}
}

func TestWriteEnvFile_SortedOutput(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{
		"ZEBRA": "z",
		"ALPHA": "a",
		"MANGO": "m",
	}
	defs := map[string]model.EnvVarDef{}

	err := runner.WriteEnvFile(dir, ".env", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Should be ALPHA, MANGO, ZEBRA with blank lines between
	varLines := []string{}
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "#") {
			varLines = append(varLines, line)
		}
	}
	if len(varLines) != 3 {
		t.Fatalf("expected 3 var lines, got %d: %v", len(varLines), varLines)
	}
	if varLines[0] != "ALPHA=a" {
		t.Errorf("first var = %q, want %q", varLines[0], "ALPHA=a")
	}
	if varLines[1] != "MANGO=m" {
		t.Errorf("second var = %q, want %q", varLines[1], "MANGO=m")
	}
	if varLines[2] != "ZEBRA=z" {
		t.Errorf("third var = %q, want %q", varLines[2], "ZEBRA=z")
	}
}

func TestWriteEnvFile_DefaultFilename(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{"KEY": "value"}
	defs := map[string]model.EnvVarDef{}

	err := runner.WriteEnvFile(dir, "", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should create .env (default filename).
	if _, err := os.Stat(filepath.Join(dir, ".env")); os.IsNotExist(err) {
		t.Error("expected .env file to exist")
	}
}

func TestWriteEnvFile_CustomFilename(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{"KEY": "value"}
	defs := map[string]model.EnvVarDef{}

	err := runner.WriteEnvFile(dir, ".env.local", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".env.local")); os.IsNotExist(err) {
		t.Error("expected .env.local file to exist")
	}
}

func TestWriteEnvFile_EmptyVars(t *testing.T) {
	dir := t.TempDir()
	vars := map[string]string{}
	defs := map[string]model.EnvVarDef{}

	err := runner.WriteEnvFile(dir, ".env", vars, defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}
