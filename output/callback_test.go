package output_test

import (
	"testing"

	"github.com/dibbla-agents/dibbla-tasks/model"
	"github.com/dibbla-agents/dibbla-tasks/output"
)

func TestCallbackFormatter_StartStep(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.StartStep("build", "Build Project")

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != model.EventTypeStatus {
		t.Errorf("Type = %q, want %q", e.Type, model.EventTypeStatus)
	}
	if e.StepID != "build" {
		t.Errorf("StepID = %q, want %q", e.StepID, "build")
	}
	if e.StepName != "Build Project" {
		t.Errorf("StepName = %q, want %q", e.StepName, "Build Project")
	}
	if e.Status != model.StepStatusRunning {
		t.Errorf("Status = %q, want %q", e.Status, model.StepStatusRunning)
	}
	if e.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestCallbackFormatter_EndStep_Success(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.EndStep("build", "Build Project", true)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventTypeStatus {
		t.Errorf("Type = %q, want %q", events[0].Type, model.EventTypeStatus)
	}
	if events[0].Status != model.StepStatusSuccess {
		t.Errorf("Status = %q, want %q", events[0].Status, model.StepStatusSuccess)
	}
}

func TestCallbackFormatter_EndStep_Failure(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.EndStep("test", "Run Tests", false)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Status != model.StepStatusFailed {
		t.Errorf("Status = %q, want %q", events[0].Status, model.StepStatusFailed)
	}
}

func TestCallbackFormatter_StdoutCarriesStepID(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.Stdout("build", "compiling main.go")

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventTypeStdout {
		t.Errorf("Type = %q, want %q", events[0].Type, model.EventTypeStdout)
	}
	if events[0].StepID != "build" {
		t.Errorf("StepID = %q, want %q", events[0].StepID, "build")
	}
	if events[0].Output != "compiling main.go" {
		t.Errorf("Output = %q, want %q", events[0].Output, "compiling main.go")
	}
}

func TestCallbackFormatter_StderrCarriesStepID(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.Stderr("build", "warning: unused variable")

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventTypeStderr {
		t.Errorf("Type = %q, want %q", events[0].Type, model.EventTypeStderr)
	}
	if events[0].StepID != "build" {
		t.Errorf("StepID = %q, want %q", events[0].StepID, "build")
	}
}

func TestCallbackFormatter_ErrorCarriesMessage(t *testing.T) {
	var events []model.StepEvent
	f := output.NewCallbackFormatter(func(e model.StepEvent) {
		events = append(events, e)
	})

	f.Error("Step Failed", "exit code 1")

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != model.EventTypeError {
		t.Errorf("Type = %q, want %q", events[0].Type, model.EventTypeError)
	}
	if events[0].Error != "Step Failed: exit code 1" {
		t.Errorf("Error = %q, want %q", events[0].Error, "Step Failed: exit code 1")
	}
}

func TestMultiFormatter_DelegatesToAll(t *testing.T) {
	var events1, events2 []model.StepEvent
	f1 := output.NewCallbackFormatter(func(e model.StepEvent) {
		events1 = append(events1, e)
	})
	f2 := output.NewCallbackFormatter(func(e model.StepEvent) {
		events2 = append(events2, e)
	})
	multi := output.NewMultiFormatter(f1, f2)

	multi.StartStep("x", "X")
	multi.EndStep("x", "X", true)
	multi.Error("E", "err")
	multi.Warning("W", "warn")
	multi.Notice("N", "note")
	multi.Debug("dbg")
	multi.Stdout("x", "line")
	multi.Stderr("x", "err line")

	if len(events1) != 8 {
		t.Errorf("events1 count = %d, want 8", len(events1))
	}
	if len(events2) != 8 {
		t.Errorf("events2 count = %d, want 8", len(events2))
	}
}
