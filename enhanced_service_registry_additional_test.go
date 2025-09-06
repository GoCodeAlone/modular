package modular

import (
	"reflect"
	"testing"
)

// Additional coverage for EnhancedServiceRegistry edge cases not exercised in the main test suite.

// Test that nil service entries are safely skipped during interface discovery.
type enhancedIface interface{ TestMethod() string }
type enhancedImpl struct{}

func (i *enhancedImpl) TestMethod() string { return "ok" }

func TestEnhancedServiceRegistry_NilServiceSkippedInInterfaceDiscovery(t *testing.T) {
	registry := NewEnhancedServiceRegistry()

	// Register a real service implementing the interface
	realSvc := &enhancedImpl{}
	if _, err := registry.RegisterService("real", realSvc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually insert a nil service entry simulating a module that attempted to register a nil
	// (application logic normally shouldn't do this, but we guard against it defensively)
	registry.services["nilService"] = &ServiceRegistryEntry{ // direct insertion to hit skip branch
		Service:      nil,
		ModuleName:   "mod",
		OriginalName: "nilService",
		ActualName:   "nilService",
	}

	entries := registry.GetServicesByInterface(reflect.TypeOf((*enhancedIface)(nil)).Elem())
	if len(entries) != 1 {
		t.Fatalf("expected only the non-nil service to be returned, got %d", len(entries))
	}
	if entries[0].ActualName != "real" {
		t.Fatalf("expected 'real' service, got %s", entries[0].ActualName)
	}
}

// Test that the backwards-compatible map returned by AsServiceRegistry is a copy
// and mutating it does not affect the internal registry state.
func TestEnhancedServiceRegistry_AsServiceRegistryIsolation(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	if _, err := registry.RegisterService("svc", "value"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	compat := registry.AsServiceRegistry()
	// Mutate the returned map
	compat["svc"] = "changed"
	compat["newsvc"] = 123

	// Internal entry should remain unchanged
	internal, ok := registry.GetService("svc")
	if !ok || internal != "value" {
		t.Fatalf("internal registry mutated; got %v, ok=%v", internal, ok)
	}

	// Newly added key should not exist internally
	if _, exists := registry.GetService("newsvc"); exists {
		t.Fatalf("unexpected newsvc present internally")
	}
}

// Test retrieval of services by a module name that has not registered services.
func TestEnhancedServiceRegistry_GetServicesByModuleEmpty(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	// No registrations for module "ghost"
	services := registry.GetServicesByModule("ghost")
	if len(services) != 0 {
		t.Fatalf("expected empty slice for unknown module, got %d", len(services))
	}
}
