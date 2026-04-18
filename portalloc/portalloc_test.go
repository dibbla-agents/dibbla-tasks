package portalloc

import (
	"net"
	"strconv"
	"testing"
)

func TestAllocate_PreferredFree(t *testing.T) {
	// Use a high port that is very likely free.
	def := PortDef{Preferred: 18923}
	port, err := Allocate(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 18923 {
		t.Fatalf("expected preferred port 18923, got %d", port)
	}
}

func TestAllocate_PreferredBusy(t *testing.T) {
	// Occupy the preferred port.
	ln, err := net.Listen("tcp", ":18924")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer ln.Close()

	def := PortDef{Preferred: 18924}
	port, err := Allocate(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 18924 {
		t.Fatal("should not have returned the occupied port")
	}
	if port < 18925 || port > 19024 {
		// Should be in the scan range (preferred+1 through preferred+100).
		t.Fatalf("expected port in range 18925-19024, got %d", port)
	}
}

func TestAllocate_Excluded(t *testing.T) {
	excluded := map[int]bool{18925: true}
	def := PortDef{Preferred: 18925}
	port, err := Allocate(def, excluded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port == 18925 {
		t.Fatal("should not have returned excluded port")
	}
}

func TestAllocate_CustomRange(t *testing.T) {
	// Occupy preferred port, set range to 2.
	ln, err := net.Listen("tcp", ":18926")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer ln.Close()

	def := PortDef{Preferred: 18926, Range: 2}
	port, err := Allocate(def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port < 18927 || port > 18928 {
		// With range=2 and preferred busy, it should try 18927 and 18928
		// If both are also busy, it falls back to OS. But they should be free.
		t.Fatalf("expected port 18927 or 18928, got %d", port)
	}
}

func TestAllocateAll_NoDuplicates(t *testing.T) {
	defs := map[string]PortDef{
		"PORT_A": {Preferred: 18930},
		"PORT_B": {Preferred: 18930}, // Same preferred — should get different actual ports.
	}
	allocated, err := AllocateAll(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocated["PORT_A"] == allocated["PORT_B"] {
		t.Fatalf("both ports got %d — should be different", allocated["PORT_A"])
	}
}

func TestAllocateAll_Deterministic(t *testing.T) {
	defs := map[string]PortDef{
		"ALPHA": {Preferred: 18940},
		"BETA":  {Preferred: 18941},
	}
	// Run twice and check we get the same results (sorted name order).
	a1, err := AllocateAll(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a2, err := AllocateAll(defs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a1["ALPHA"] != a2["ALPHA"] || a1["BETA"] != a2["BETA"] {
		t.Fatalf("non-deterministic: run1=%v run2=%v", a1, a2)
	}
}

func TestAllocate_FallbackToOS(t *testing.T) {
	// Occupy a tiny range completely.
	port := 18950
	listeners := make([]net.Listener, 0, 3)
	for i := 0; i <= 2; i++ {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port+i))
		if err != nil {
			// Port might be in use by something else; skip.
			t.Skipf("cannot occupy port %d: %v", port+i, err)
		}
		listeners = append(listeners, ln)
	}
	defer func() {
		for _, ln := range listeners {
			ln.Close()
		}
	}()

	def := PortDef{Preferred: port, Range: 2}
	got, err := Allocate(def, nil)
	if err != nil {
		t.Fatalf("expected OS fallback to succeed, got error: %v", err)
	}
	// Should be an ephemeral port outside our range.
	if got >= port && got <= port+2 {
		t.Fatalf("expected port outside occupied range, got %d", got)
	}
}
