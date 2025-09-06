package eventbus

import (
	"testing"
)

// TestGetRegisteredEngines verifies custom engine registration appears in list.
func TestGetRegisteredEngines(t *testing.T) {
	engines := GetRegisteredEngines()
	if len(engines) == 0 {
		t.Fatalf("expected at least one registered engine")
	}
	// ensure known built-in engines appear (memory) and custom engine factory also present if registered under name "custom" or "custom-memory"
	hasMemory := false
	hasCustomVariant := false
	for _, e := range engines {
		if e == "memory" {
			hasMemory = true
		}
		if e == "custom" || e == "custom-memory" {
			hasCustomVariant = true
		}
	}
	if !hasMemory {
		t.Fatalf("expected memory engine present: %v", engines)
	}
	if !hasCustomVariant {
		t.Fatalf("expected custom engine present (custom or custom-memory) in %v", engines)
	}
}
