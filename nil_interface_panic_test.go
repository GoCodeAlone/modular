package modular

import (
	"reflect"
	"testing"
)

// TestNilServiceInstancePanic reproduces the nil pointer panic issue
// when interface-based matching encounters a service with nil Instance
func TestNilServiceInstancePanic(t *testing.T) {
	// Create a module that provides a service with nil Instance
	nilServiceModule := &nilServiceProviderModule{}
	
	// Create a module that requires an interface-based service
	consumerModule := &interfaceConsumerModule{}
	
	// Create app with proper logger to avoid other nil pointer issues
	logger := &mockTestLogger{}
	app := NewStdApplication(nil, logger)
	app.RegisterModule(nilServiceModule)
	app.RegisterModule(consumerModule)
	
	// This should not panic, even with nil service instance
	err := app.Init()
	if err != nil {
		t.Logf("Init error (expected due to nil service but should not panic): %v", err)
	}
	
	// Test should pass if no panic occurs
	t.Log("✅ No panic occurred during initialization with nil service instance")
}

// TestTypeImplementsInterfaceWithNil tests the typeImplementsInterface function with nil types
func TestTypeImplementsInterfaceWithNil(t *testing.T) {
	app := &StdApplication{}
	
	// Test with nil svcType (should not panic)
	interfaceType := reflect.TypeOf((*NilTestInterface)(nil)).Elem()
	result := app.typeImplementsInterface(nil, interfaceType)
	if result {
		t.Error("Expected false when svcType is nil")
	}
	
	// Test with nil interfaceType (should not panic)
	svcType := reflect.TypeOf("")
	result = app.typeImplementsInterface(svcType, nil)
	if result {
		t.Error("Expected false when interfaceType is nil")
	}
	
	// Test with both nil (should not panic)
	result = app.typeImplementsInterface(nil, nil)
	if result {
		t.Error("Expected false when both types are nil")
	}
	
	t.Log("✅ typeImplementsInterface handles nil types without panic")
}

// TestGetServicesByInterfaceWithNilService tests GetServicesByInterface with nil services
func TestGetServicesByInterfaceWithNilService(t *testing.T) {
	app := NewStdApplication(nil, nil)
	
	// Register a service with nil instance
	err := app.RegisterService("nilService", nil)
	if err != nil {
		t.Fatalf("Failed to register nil service: %v", err)
	}
	
	// This should not panic
	interfaceType := reflect.TypeOf((*NilTestInterface)(nil)).Elem()
	results := app.GetServicesByInterface(interfaceType)
	
	// Should return empty results, not panic
	if len(results) != 0 {
		t.Errorf("Expected no results for interface match with nil service, got %d", len(results))
	}
	
	t.Log("✅ GetServicesByInterface handles nil services without panic")
}

// Test interface for the tests
type NilTestInterface interface {
	TestMethod()
}

// nilServiceProviderModule provides a service with nil Instance
type nilServiceProviderModule struct{}

func (m *nilServiceProviderModule) Name() string {
	return "nil-service-provider"
}

func (m *nilServiceProviderModule) Init(app Application) error {
	return nil
}

func (m *nilServiceProviderModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "nilService",
		Instance: nil, // Intentionally nil
	}}
}

// interfaceConsumerModule requires an interface-based service
type interfaceConsumerModule struct{}

func (m *interfaceConsumerModule) Name() string {
	return "interface-consumer"
}

func (m *interfaceConsumerModule) Init(app Application) error {
	return nil
}

func (m *interfaceConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "testService",
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeOf((*NilTestInterface)(nil)).Elem(),
		Required:           false, // Make it optional to avoid required service errors
	}}
}