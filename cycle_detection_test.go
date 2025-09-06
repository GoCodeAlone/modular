package modular

import (
	"reflect"
	"testing"
)

// Test interface for cycle detection
type TestInterface interface {
	TestMethod() string
}

// Mock modules for cycle detection testing

// CycleTestModuleA provides TestInterface and depends on CycleTestModuleB via interface
type CycleTestModuleA struct {
	name string
}

func (m *CycleTestModuleA) Name() string {
	return m.name
}

func (m *CycleTestModuleA) Init(app Application) error {
	return nil
}

func (m *CycleTestModuleA) Dependencies() []string {
	return []string{} // No module dependencies
}

func (m *CycleTestModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:     "testServiceA",
			Instance: &TestServiceImpl{name: "A"},
		},
	}
}

func (m *CycleTestModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "testServiceB",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*TestInterface)(nil)).Elem(),
		},
	}
}

// CycleTestModuleB provides TestInterface and depends on CycleTestModuleA via interface
type CycleTestModuleB struct {
	name string
}

func (m *CycleTestModuleB) Name() string {
	return m.name
}

func (m *CycleTestModuleB) Init(app Application) error {
	return nil
}

func (m *CycleTestModuleB) Dependencies() []string {
	return []string{} // No module dependencies
}

func (m *CycleTestModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:     "testServiceB",
			Instance: &TestServiceImpl{name: "B"},
		},
	}
}

func (m *CycleTestModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "testServiceA",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*TestInterface)(nil)).Elem(),
		},
	}
}

// TestServiceImpl implements TestInterface
type TestServiceImpl struct {
	name string
}

func (t *TestServiceImpl) TestMethod() string {
	return t.name
}

// Test that cycle detection works with interface-based dependencies
func TestCycleDetectionWithInterfaceDependencies(t *testing.T) {
	// Create application with two modules that have circular interface dependencies
	logger := &testLogger{}

	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         logger,
	}

	// Register modules
	moduleA := &CycleTestModuleA{name: "moduleA"}
	moduleB := &CycleTestModuleB{name: "moduleB"}

	app.RegisterModule(moduleA)
	app.RegisterModule(moduleB)

	// Attempt to initialize - should detect cycle
	err := app.Init()
	if err == nil {
		t.Error("Expected cycle detection error, but got none")
		return
	}

	// Check that the error message includes cycle information and interface details
	if !IsErrCircularDependency(err) {
		t.Errorf("Expected ErrCircularDependency, got %T: %v", err, err)
	}

	errStr := err.Error()
	t.Logf("Cycle detection error: %s", errStr)

	// Verify the error message contains useful information
	if !containsString(errStr, "cycle:") {
		t.Error("Expected error message to contain 'cycle:'")
	}

	// Should contain both module names
	if !containsString(errStr, "moduleA") || !containsString(errStr, "moduleB") {
		t.Error("Expected error message to contain both module names")
	}

	// Should indicate interface-based dependency
	if !containsString(errStr, "interface:") {
		t.Error("Expected error message to indicate interface-based dependency")
	}
}

// Test helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCycleDetectionWithMixedDependencies tests that non-circular dependencies work correctly
func TestCycleDetectionWithNonCircularDependencies(t *testing.T) {
	logger := &testLogger{}

	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         logger,
	}

	// Create a simple module without dependencies
	simpleModule := &SimpleModule{name: "simpleModule"}

	app.RegisterModule(simpleModule)

	// This should initialize without any issues
	err := app.Init()
	if err != nil {
		t.Errorf("Unexpected error during initialization: %v", err)
	}
}

// Simple module without dependencies for testing
type SimpleModule struct {
	name string
}

func (m *SimpleModule) Name() string {
	return m.name
}

func (m *SimpleModule) Init(app Application) error {
	return nil
}

func (m *SimpleModule) Dependencies() []string {
	return nil
}

func (m *SimpleModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *SimpleModule) RequiresServices() []ServiceDependency {
	return nil
}

// TestEdgeTypeString tests the EdgeType string representation
func TestEdgeTypeString(t *testing.T) {
	tests := []struct {
		edgeType EdgeType
		expected string
	}{
		{EdgeTypeModule, "module"},
		{EdgeTypeNamedService, "named-service"},
		{EdgeTypeInterfaceService, "interface-service"},
		{EdgeType(999), "unknown"},
	}

	for _, test := range tests {
		result := test.edgeType.String()
		if result != test.expected {
			t.Errorf("EdgeType(%d).String() = %s, expected %s", test.edgeType, result, test.expected)
		}
	}
}
