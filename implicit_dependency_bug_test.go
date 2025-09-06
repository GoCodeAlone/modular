package modular

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

// Static errors for linter compliance
var (
	ErrRouterServiceNotFound        = errors.New("required service 'router' not found or does not implement http.Handler")
	ErrCustomServiceNotImplementing = errors.New("required service not found or does not implement http.Handler")
)

// TestImplicitDependencyFlakiness tests the non-deterministic behavior
// when modules have implicit interface-based dependencies but no explicit dependencies.
// This reproduces the issue where httpserver sometimes fails to find the router service
// provided by chimux due to non-deterministic initialization order.
func TestImplicitDependencyFlakiness(t *testing.T) {
	// Test the specific case where implicit dependency detection fails
	// by manipulating the module order to put consumer before provider
	err := runForcedBadOrderTest()
	if err == nil {
		t.Error("Expected test to fail with forced bad order, but it succeeded - implicit dependency detection may not be working")
	} else {
		t.Logf("Successfully reproduced the bug: %v", err)
	}

	// Test that the normal Init() method now works correctly
	err = runNormalInitTest()
	if err != nil {
		t.Errorf("Normal initialization should work after fix, but failed: %v", err)
	} else {
		t.Log("Normal initialization works correctly with the fix")
	}
}

// runNormalInitTest tests that the fixed initialization logic works correctly
func runNormalInitTest() error {
	// Create test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Create modules
	routerProvider := &FlakyChiMuxModule{name: "chimux"}
	serverConsumer := &FlakyServerModule{name: "httpserver"}

	app.RegisterModule(routerProvider)
	app.RegisterModule(serverConsumer)

	// Use the normal (now fixed) Init method
	return app.Init()
}

// runForcedBadOrderTest forces a bad initialization order to reproduce the bug
func runForcedBadOrderTest() error {
	// Create test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Create modules
	routerProvider := &FlakyChiMuxModule{name: "chimux"}
	serverConsumer := &FlakyServerModule{name: "httpserver"}

	app.RegisterModule(routerProvider)
	app.RegisterModule(serverConsumer)

	// Force bad order by initializing consumer before provider
	// This simulates what happens when dependency resolution fails
	return app.initWithForcedBadOrder()
}

// TestImplicitDependencyDeterministicFix tests that after fixing the dependency resolution,
// it becomes deterministic and always works correctly.
func TestImplicitDependencyDeterministicFix(t *testing.T) {
	// This test will pass once we fix the dependency resolution to be deterministic
	attempts := 20

	for i := 0; i < attempts; i++ {
		err := runSingleImplicitDependencyTestWithFix()
		if err != nil {
			t.Fatalf("Attempt %d failed after fix: %v", i+1, err)
		}
	}

	t.Logf("All %d attempts succeeded after fix", attempts)
}

// runSingleImplicitDependencyTestWithFix runs the test with the fixed dependency resolution
func runSingleImplicitDependencyTestWithFix() error {
	// Create test application with the fixed dependency resolution
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Create the same modules as in the flaky test
	routerProvider := &FlakyChiMuxModule{name: "chimux"}
	serverConsumer := &FlakyServerModule{name: "httpserver"}

	app.RegisterModule(routerProvider)
	app.RegisterModule(serverConsumer)

	// Initialize using the normal (now fixed) dependency resolution
	err := app.Init()
	if err != nil {
		return fmt.Errorf("application initialization failed: %w", err)
	}

	return nil
}

// TestNamingGameAttempt tests that clever module naming cannot break the deterministic dependency resolution.
// This test uses module names specifically designed to try to "game" the alphabetical sorting
// and potentially cause the consumer to be initialized before the provider.
func TestNamingGameAttempt(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		consumerName string
		description  string
	}{
		{
			name:         "AlphabeticalTrick",
			providerName: "zz_provider", // Should be last alphabetically
			consumerName: "aa_consumer", // Should be first alphabetically
			description:  "Consumer name comes before provider name alphabetically",
		},
		{
			name:         "NumericTrick",
			providerName: "9_provider", // Numeric prefix to be last
			consumerName: "1_consumer", // Numeric prefix to be first
			description:  "Consumer has numeric prefix that sorts before provider",
		},
		{
			name:         "UnicodeEdgeCase",
			providerName: "zzz_provider", // Multiple z's to be very last
			consumerName: "000_consumer", // Zeros to be very first
			description:  "Extreme alphabetical separation between provider and consumer",
		},
		{
			name:         "SimilarNamesEdgeCase",
			providerName: "service_provider_x", // Longer name
			consumerName: "service_provider",   // Shorter but similar name (should sort first)
			description:  "Similar names where shorter name sorts before longer name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test multiple times to ensure deterministic behavior
			for attempt := 0; attempt < 5; attempt++ {
				err := runNamingGameTest(tt.providerName, tt.consumerName)
				if err != nil {
					t.Errorf("Attempt %d failed for %s (%s): %v",
						attempt+1, tt.name, tt.description, err)
				}
			}
		})
	}
}

// runNamingGameTest runs a test with specific module names to try to break deterministic ordering
func runNamingGameTest(providerName, consumerName string) error {
	// Create test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Create modules with the specified names
	routerProvider := &FlakyChiMuxModule{name: providerName}
	serverConsumer := &FlakyServerModule{name: consumerName}

	// Register in a potentially problematic order (consumer first)
	app.RegisterModule(serverConsumer)
	app.RegisterModule(routerProvider)

	// The fixed dependency resolution should handle this correctly regardless of naming
	return app.Init()
}

// TestServiceNamingGameAttempt tests that clever service naming cannot break dependency resolution.
// This focuses on service names rather than module names.
func TestServiceNamingGameAttempt(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		description string
	}{
		{
			name:        "EmptyStringService",
			serviceName: "", // Edge case: empty service name
			description: "Empty service name should not break resolution",
		},
		{
			name:        "NumericService",
			serviceName: "0000_router", // Numeric prefix
			description: "Numeric service name prefix",
		},
		{
			name:        "UnicodeService",
			serviceName: "zzz_router", // Late alphabetically
			description: "Service name that sorts very late",
		},
		{
			name:        "SpecialCharsService",
			serviceName: "___router", // Special characters
			description: "Service name with special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle empty service name test case specifically
			if tt.serviceName == "" {
				// Test that empty service names are handled gracefully
				err := runServiceNamingGameTest(tt.serviceName)
				// Empty service names should either:
				// 1. Be allowed and work correctly (if the system handles them)
				// 2. Return a specific error (if they're invalid)
				// For now, let's test that the system doesn't crash and handles them consistently

				// Test multiple times to ensure deterministic behavior even with empty names
				for attempt := 1; attempt < 3; attempt++ {
					err2 := runServiceNamingGameTest(tt.serviceName)
					// The behavior should be consistent across attempts
					if (err == nil) != (err2 == nil) {
						t.Errorf("Inconsistent behavior with empty service name: attempt 1 error=%v, attempt %d error=%v", err, attempt+1, err2)
					}
				}

				// The test passes as long as the behavior is consistent and doesn't crash
				return
			}

			// Test multiple times to ensure deterministic behavior
			for attempt := 0; attempt < 3; attempt++ {
				err := runServiceNamingGameTest(tt.serviceName)
				if err != nil {
					t.Errorf("Attempt %d failed for %s (%s): %v",
						attempt+1, tt.name, tt.description, err)
				}
			}
		})
	}
}

// runServiceNamingGameTest runs a test with a specific service name to try to break resolution
func runServiceNamingGameTest(serviceName string) error {
	// Create test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Create modules with the specified service name
	routerProvider := &CustomServiceNameModule{name: "provider", serviceName: serviceName}
	serverConsumer := &CustomServiceConsumerModule{name: "consumer", serviceName: serviceName}

	// Register in potentially problematic order
	app.RegisterModule(serverConsumer)
	app.RegisterModule(routerProvider)

	// The fixed dependency resolution should handle this correctly
	return app.Init()
}

// FlakyChiMuxModule simulates the chimux module
type FlakyChiMuxModule struct {
	name string
}

func (m *FlakyChiMuxModule) Name() string {
	return m.name
}

func (m *FlakyChiMuxModule) Init(_ Application) error {
	return nil
}

func (m *FlakyChiMuxModule) Dependencies() []string {
	// No explicit dependencies - this is key to reproducing the bug
	return nil
}

func (m *FlakyChiMuxModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "router",
			Description: "HTTP router service",
			Instance:    m, // The module itself implements http.Handler
		},
	}
}

func (m *FlakyChiMuxModule) RequiresServices() []ServiceDependency {
	return nil
}

// Implement http.Handler to satisfy the interface that FlakyServerModule requires
func (m *FlakyChiMuxModule) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Stub implementation
}

// FlakyServerModule simulates the httpserver module
type FlakyServerModule struct {
	name    string
	handler http.Handler
}

func (m *FlakyServerModule) Name() string {
	return m.name
}

func (m *FlakyServerModule) Init(_ Application) error {
	return nil
}

func (m *FlakyServerModule) Dependencies() []string {
	// No explicit dependencies - this is key to reproducing the bug
	return nil
}

func (m *FlakyServerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *FlakyServerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*http.Handler)(nil)).Elem(),
		},
	}
}

func (m *FlakyServerModule) Constructor() ModuleConstructor {
	return func(_ Application, services map[string]any) (Module, error) {
		if handler, ok := services["router"].(http.Handler); ok {
			m.handler = handler
			return m, nil
		}
		return nil, ErrRouterServiceNotFound
	}
}

// testLogger is a simple logger implementation for testing
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...any) {}
func (l *testLogger) Info(msg string, keysAndValues ...any)  {}
func (l *testLogger) Warn(msg string, keysAndValues ...any)  {}
func (l *testLogger) Error(msg string, keysAndValues ...any) {}

// initWithForcedBadOrder forces a bad initialization order to demonstrate the bug
func (app *StdApplication) initWithForcedBadOrder() error {
	// Hardcode a bad order: consumer before provider
	order := []string{"httpserver", "chimux"}

	// Initialize modules in the bad order
	for _, name := range order {
		module := app.moduleRegistry[name]

		// Try to inject services BEFORE the provider has registered them
		if _, ok := module.(ServiceAware); ok {
			module, err := app.injectServices(module)
			if err != nil {
				return fmt.Errorf("failed to inject services for module %s: %w", name, err)
			}
			app.moduleRegistry[name] = module
		}

		if err := module.Init(app); err != nil {
			return fmt.Errorf("failed to initialize module %s: %w", name, err)
		}

		// Register services provided by modules AFTER initialization
		if svcAware, ok := module.(ServiceAware); ok {
			for _, svc := range svcAware.ProvidesServices() {
				if err := app.RegisterService(svc.Name, svc.Instance); err != nil {
					return fmt.Errorf("module '%s' failed to register service '%s': %w", name, svc.Name, err)
				}
			}
		}
	}

	return nil
}

// CustomServiceNameModule is a module that provides a service with a custom name
type CustomServiceNameModule struct {
	name        string
	serviceName string
}

func (m *CustomServiceNameModule) Name() string {
	return m.name
}

func (m *CustomServiceNameModule) Init(_ Application) error {
	return nil
}

func (m *CustomServiceNameModule) Dependencies() []string {
	return nil
}

func (m *CustomServiceNameModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        m.serviceName,
			Description: "Custom named HTTP router service",
			Instance:    m,
		},
	}
}

func (m *CustomServiceNameModule) RequiresServices() []ServiceDependency {
	return nil
}

func (m *CustomServiceNameModule) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Stub implementation
}

// CustomServiceConsumerModule is a module that consumes a service with a custom name
type CustomServiceConsumerModule struct {
	name        string
	serviceName string
	handler     http.Handler
}

func (m *CustomServiceConsumerModule) Name() string {
	return m.name
}

func (m *CustomServiceConsumerModule) Init(_ Application) error {
	return nil
}

func (m *CustomServiceConsumerModule) Dependencies() []string {
	return nil
}

func (m *CustomServiceConsumerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *CustomServiceConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               m.serviceName,
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*http.Handler)(nil)).Elem(),
		},
	}
}

func (m *CustomServiceConsumerModule) Constructor() ModuleConstructor {
	return func(_ Application, services map[string]any) (Module, error) {
		if handler, ok := services[m.serviceName].(http.Handler); ok {
			m.handler = handler
			return m, nil
		}
		return nil, fmt.Errorf("required service '%s': %w", m.serviceName, ErrCustomServiceNotImplementing)
	}
}

// TestAlphabeticalNamingBreaker tests that dependency resolution works correctly
// even when module and service names would break alphabetical sorting.
// This ensures we're doing actual dependency resolution, not just alphabetical ordering.
func TestAlphabeticalNamingBreaker(t *testing.T) {
	t.Log("Testing dependency resolution with names designed to break alphabetical sorting...")

	// Test case 1: Provider module name comes AFTER consumer alphabetically
	err := runAlphabeticalBreakerTest1()
	if err != nil {
		t.Errorf("Alphabetical breaker test 1 failed: %v", err)
	} else {
		t.Log("✓ Test 1 passed: Provider with later alphabetical name works correctly")
	}

	// Test case 2: Even more extreme alphabetical violation
	err = runAlphabeticalBreakerTest2()
	if err != nil {
		t.Errorf("Alphabetical breaker test 2 failed: %v", err)
	} else {
		t.Log("✓ Test 2 passed: Extreme alphabetical violation handled correctly")
	}

	// Test case 3: Multiple providers with bad alphabetical order
	err = runAlphabeticalBreakerTest3()
	if err != nil {
		t.Errorf("Alphabetical breaker test 3 failed: %v", err)
	} else {
		t.Log("✓ Test 3 passed: Multiple providers with bad alphabetical order work correctly")
	}
}

// runAlphabeticalBreakerTest1 tests provider module name comes after consumer alphabetically
func runAlphabeticalBreakerTest1() error {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// DELIBERATE: Consumer comes first alphabetically, provider comes last
	// If we're just doing alphabetical sorting, this would fail
	consumerModule := &FlakyServerModule{name: "aaa-consumer-server"} // Starts with 'a'
	providerModule := &FlakyChiMuxModule{name: "zzz-provider-router"} // Starts with 'z'

	app.RegisterModule(consumerModule)
	app.RegisterModule(providerModule)

	return app.Init()
}

// runAlphabeticalBreakerTest2 tests even more extreme alphabetical violation
func runAlphabeticalBreakerTest2() error {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// EXTREME CASE: Numbers to force specific alphabetical order
	// Consumer: starts with "1"
	// Provider: starts with "9"
	consumerModule := &FlakyServerModule{name: "1-first-consumer"}
	providerModule := &FlakyChiMuxModule{name: "9-last-provider"}

	app.RegisterModule(consumerModule)
	app.RegisterModule(providerModule)

	return app.Init()
}

// runAlphabeticalBreakerTest3 tests single consumer with multiple potential providers (but only one registers)
func runAlphabeticalBreakerTest3() error {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &testLogger{},
	}

	// Single consumer with alphabetically-first name, single provider with alphabetically-last name
	// Add some other modules that don't provide the needed service to test proper filtering
	consumer := &FlakyServerModule{name: "aaa-consumer"}
	provider := &FlakyChiMuxModule{name: "zzz-provider"}
	dummy1 := &DummyModule{name: "mmm-dummy1"}
	dummy2 := &DummyModule{name: "nnn-dummy2"}

	// Register in order that would break alphabetical dependency resolution
	app.RegisterModule(consumer)
	app.RegisterModule(dummy1)
	app.RegisterModule(dummy2)
	app.RegisterModule(provider)

	return app.Init()
}

// DummyModule is a module that doesn't provide any services, used for testing
type DummyModule struct {
	name string
}

func (m *DummyModule) Name() string {
	return m.name
}

func (m *DummyModule) Dependencies() []string {
	return nil // No dependencies
}

func (m *DummyModule) Init(app Application) error {
	// Initialize but don't register any services
	return nil
}
