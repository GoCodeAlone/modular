package modular

import (
	"reflect"
	"testing"
)

// TestUserScenarioReproduction tests the exact scenario from issue #88
func TestUserScenarioReproduction(t *testing.T) {
	// Test the nil service instance scenario that caused the panic
	nilServiceModule := &testNilServiceModule{}
	consumerModule := &testInterfaceConsumerModule{}

	app := NewStdApplication(nil, &mockTestLogger{})
	app.RegisterModule(nilServiceModule)
	app.RegisterModule(consumerModule)

	// This should not panic - the main fix
	err := app.Init()
	if err != nil {
		t.Logf("Init completed with expected error (but no panic): %v", err)
	} else {
		t.Log("Init completed successfully")
	}

	// Verify the enhanced service registry methods work
	services := app.ServiceIntrospector().GetServicesByModule("nil-service")
	t.Logf("Services from nil-service module: %v", services)

	entry, found := app.ServiceIntrospector().GetServiceEntry("nilService")
	if found {
		t.Logf("Found service entry: %+v", entry)
	} else {
		t.Log("Service entry not found (expected for nil service)")
	}

	interfaceType := reflect.TypeOf((*TestUserInterface)(nil)).Elem()
	interfaceServices := app.ServiceIntrospector().GetServicesByInterface(interfaceType)
	t.Logf("Services implementing interface: %d", len(interfaceServices))

	t.Log("✅ User scenario completed without panic")
}

// TestBackwardsCompatibilityCheck verifies that existing mock applications need updates
func TestBackwardsCompatibilityCheck(t *testing.T) {
	// This test verifies the new interface methods exist
	var app Application = NewStdApplication(nil, &mockTestLogger{})

	// Test that new methods are available and don't panic
	services := app.ServiceIntrospector().GetServicesByModule("nonexistent")
	if len(services) != 0 {
		t.Errorf("Expected empty services for nonexistent module, got %v", services)
	}

	entry, found := app.ServiceIntrospector().GetServiceEntry("nonexistent")
	if found || entry != nil {
		t.Errorf("Expected no entry for nonexistent service, got %v, %v", entry, found)
	}

	interfaceType := reflect.TypeOf((*TestUserInterface)(nil)).Elem()
	interfaceServices := app.ServiceIntrospector().GetServicesByInterface(interfaceType)
	if len(interfaceServices) != 0 {
		t.Errorf("Expected no interface services, got %v", interfaceServices)
	}

	t.Log("✅ New interface methods work correctly")
}

// TestUserInterface matches the interface from the user's issue
type TestUserInterface interface {
	TestMethod()
}

// testNilServiceModule provides a service with nil Instance (reproduces the issue)
type testNilServiceModule struct{}

func (m *testNilServiceModule) Name() string               { return "nil-service" }
func (m *testNilServiceModule) Init(app Application) error { return nil }
func (m *testNilServiceModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "nilService",
		Instance: nil, // This is what caused the original panic
	}}
}

// testInterfaceConsumerModule consumes interface-based services (triggers the matching)
type testInterfaceConsumerModule struct{}

func (m *testInterfaceConsumerModule) Name() string               { return "consumer" }
func (m *testInterfaceConsumerModule) Init(app Application) error { return nil }
func (m *testInterfaceConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "testInterface",
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*TestUserInterface)(nil)).Elem(),
		Required:           false, // Optional to avoid initialization failures
	}}
}
