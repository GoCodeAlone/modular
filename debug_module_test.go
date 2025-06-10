package modular

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModuleReplacementLosesStartable demonstrates the bug where a module loses Startable interface
// after constructor injection
func TestModuleReplacementLosesStartable(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}
	// Create a module that implements both ServiceAware, Constructable, AND Startable
	originalModule := &ProblematicModule{name: "test-module"}

	// Verify the original module implements Startable
	_, implementsStartable := interface{}(originalModule).(Startable)
	require.True(t, implementsStartable, "Original module should implement Startable")

	// Register the module
	app.RegisterModule(originalModule)

	// Store reference to original for comparison
	originalRef := app.moduleRegistry["test-module"]

	// Initialize the application (this triggers service injection)
	err := app.Init()
	require.NoError(t, err)

	// Get the module after initialization
	moduleAfterInit := app.moduleRegistry["test-module"]

	// Compare instances
	t.Logf("Original module: %T at %p", originalRef, originalRef)
	t.Logf("Module after init: %T at %p", moduleAfterInit, moduleAfterInit)

	// Check if the module was replaced
	if originalRef != moduleAfterInit {
		t.Logf("⚠️  Module was replaced during initialization")

		// Check if the new module still implements Startable
		_, newImplementsStartable := moduleAfterInit.(Startable)
		if !newImplementsStartable {
			t.Logf("🚨 BUG DEMONSTRATED: New module instance does not implement Startable!")
			// This is the expected bug behavior - the test should pass when this happens
		} else {
			t.Errorf("Expected bug did not occur - new module should NOT implement Startable")
		}
	} else {
		t.Errorf("Expected module replacement did not occur")
	}

	// Try to start the application - this should skip the Startable module due to the bug
	err = app.Start()
	// The Start should succeed, but the module's Start method won't be called due to the bug
	require.NoError(t, err, "Application should start successfully even with the bug")

	// Verify the bug: the module should not have been started
	require.False(t, originalModule.startCalled, "BUG VERIFIED: Module's Start method should not have been called due to interface loss")
}

// TestProperModuleConstructorPattern demonstrates the correct way to implement Constructor
// that preserves all interfaces
func TestProperModuleConstructorPattern(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}
	// Create a module that properly implements Constructor pattern
	originalModule := &CorrectModule{name: "correct-module"}

	// Verify the original module implements Startable
	_, implementsStartable := interface{}(originalModule).(Startable)
	require.True(t, implementsStartable, "Original module should implement Startable")

	// Register the module
	app.RegisterModule(originalModule)

	// Initialize the application
	err := app.Init()
	require.NoError(t, err)

	// Get the module after initialization
	moduleAfterInit := app.moduleRegistry["correct-module"]

	// Check if the new module still implements Startable
	_, newImplementsStartable := moduleAfterInit.(Startable)
	assert.True(t, newImplementsStartable, "New module instance should still implement Startable")

	// Try to start the application - this should work
	err = app.Start()
	assert.NoError(t, err)
}

// ProblematicModule demonstrates a module that loses Startable interface after constructor injection
type ProblematicModule struct {
	name        string
	startCalled bool
}

func (m *ProblematicModule) Name() string           { return m.name }
func (m *ProblematicModule) Init(Application) error { return nil }

func (m *ProblematicModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *ProblematicModule) RequiresServices() []ServiceDependency { return nil }

func (m *ProblematicModule) Start(ctx context.Context) error {
	m.startCalled = true
	return nil
}

func (m *ProblematicModule) Stop(ctx context.Context) error { return nil }

// ❌ PROBLEMATIC: Constructor returns a different type that doesn't implement Startable
func (m *ProblematicModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		// 🚨 BUG: Returning a different struct that doesn't implement all the same interfaces!
		return &BrokenModuleImplementation{
			name: m.name,
			// Missing startCalled field and Start/Stop methods!
		}, nil
	}
}

// BrokenModuleImplementation only implements basic Module interface, not Startable
type BrokenModuleImplementation struct {
	name string
}

func (m *BrokenModuleImplementation) Name() string           { return m.name }
func (m *BrokenModuleImplementation) Init(Application) error { return nil }

// CorrectModule demonstrates the proper way to implement Constructor pattern
type CorrectModule struct {
	name        string
	startCalled bool
}

func (m *CorrectModule) Name() string           { return m.name }
func (m *CorrectModule) Init(Application) error { return nil }

func (m *CorrectModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *CorrectModule) RequiresServices() []ServiceDependency { return nil }

func (m *CorrectModule) Start(ctx context.Context) error {
	m.startCalled = true
	return nil
}

func (m *CorrectModule) Stop(ctx context.Context) error { return nil }

// ✅ CORRECT: Constructor returns the same instance (or a new instance that implements all interfaces)
func (m *CorrectModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		// ✅ GOOD: Return the same instance or create a new one with all interfaces
		newModule := &CorrectModule{
			name: m.name,
			// Initialize with injected services if needed
		}
		return newModule, nil
	}
}

// TestDebugModuleInterfaces tests the debugging utility
func TestDebugModuleInterfaces(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Register both types of modules
	app.RegisterModule(&ProblematicModule{name: "problematic"})
	app.RegisterModule(&CorrectModule{name: "correct"})

	t.Log("=== Before Initialization ===")
	DebugAllModuleInterfaces(app)

	// Initialize
	err := app.Init()
	require.NoError(t, err)

	t.Log("=== After Initialization ===")
	DebugAllModuleInterfaces(app)
}
