package modular

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestInterfaceMatching simulates the issue between chimux and reverseproxy modules
// without directly importing them, focusing on the interface visibility problem.
func TestInterfaceMatching(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Register modules - provider first, consumer second (opposite order to force dependency resolution)
	consumerModule := &InterfaceConsumerModule{name: "consumer"}
	providerModule := &InterfaceProviderModule{name: "provider"}

	app.RegisterModule(consumerModule)
	app.RegisterModule(providerModule)

	// Resolve dependencies
	order, err := app.resolveDependencies()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// Verify provider comes before consumer
	providerIdx := -1
	consumerIdx := -1
	for i, name := range order {
		switch name {
		case "provider":
			providerIdx = i
		case "consumer":
			consumerIdx = i
		}
	}

	if providerIdx == -1 || consumerIdx == -1 {
		t.Fatalf("Module order missing provider or consumer: %v", order)
	}

	if providerIdx > consumerIdx {
		t.Errorf("Expected provider to come before consumer, but got: %v", order)
	}

	// Register the provider's service
	err = app.RegisterService("router.service", providerModule)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Initialize provider
	err = providerModule.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Try to inject service into consumer
	injectedModule, err := app.injectServices(consumerModule)
	if err != nil {
		t.Fatalf("Failed to inject services: %v", err)
	}

	injectedConsumer := injectedModule.(*InterfaceConsumerModule)
	if injectedConsumer.router == nil {
		t.Error("Router service was not injected")
	}
}

// TestInterfaceMatchingWithDifferentNames tests interface matching when services have different names
// This is more similar to the real-world issue with chimux and reverseproxy
func TestInterfaceMatchingWithDifferentNames(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Register modules - this time with more realistic names
	chiMux := &InterfaceProviderModule{name: "chimux"}

	// Create a custom consumer that expects "router" not "router.service"
	// This simulates the reverseproxy module looking for a service named "router"
	reverseProxy := &CustomNameConsumerModule{name: "reverseproxy"}

	app.RegisterModule(chiMux)
	app.RegisterModule(reverseProxy)

	// Register the provider's service with a different name than what consumer expects
	// This better simulates the real issue where chimux registers service as "chimux.router"
	err := app.RegisterService("chimux.router", chiMux)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Initialize provider
	err = chiMux.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Try to inject service into consumer
	injectedModule, err := app.injectServices(reverseProxy)
	if err != nil {
		t.Fatalf("Failed to inject services: %v", err)
	}

	injectedConsumer := injectedModule.(*CustomNameConsumerModule)
	if injectedConsumer.router == nil {
		t.Error("Router service was not injected despite interface match")
	}
}

// TestInterfaceMatchingNegativeCase tests what happens when no service implements the required interface
func TestInterfaceMatchingNegativeCase(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Create a consumer module
	consumer := &InterfaceConsumerModule{name: "consumer"}
	app.RegisterModule(consumer)

	// Create a provider that does NOT implement the required interface
	incompatibleProvider := &IncompatibleProvider{name: "incompatible"}
	app.RegisterModule(incompatibleProvider)

	// Register the incompatible service
	err := app.RegisterService("router.service", incompatibleProvider)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Initialize provider
	err = incompatibleProvider.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Try to inject services - this should fail since the service doesn't implement the interface
	_, err = app.injectServices(consumer)
	if err == nil {
		t.Fatal("Expected error when injecting incompatible service, but got none")
	}

	// Verify error message contains information about missing interface
	expectedErrText := "no service found implementing interface"
	if !strings.Contains(err.Error(), expectedErrText) {
		t.Errorf("Error message doesn't contain expected text. Got: %v", err)
	}
}

// TestMultipleInterfaceProviders tests when multiple services implement the required interface
func TestMultipleInterfaceProviders(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Register modules
	consumer := &InterfaceConsumerModule{name: "consumer"}
	provider1 := &InterfaceProviderModule{name: "provider1"}
	provider2 := &InterfaceProviderModule{name: "provider2"}

	app.RegisterModule(consumer)
	app.RegisterModule(provider1)
	app.RegisterModule(provider2)

	// Register multiple services that implement the same interface
	err := app.RegisterService("service1", provider1)
	if err != nil {
		t.Fatalf("Failed to register service1: %v", err)
	}

	err = app.RegisterService("service2", provider2)
	if err != nil {
		t.Fatalf("Failed to register service2: %v", err)
	}

	// Initialize providers
	err = provider1.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize provider1: %v", err)
	}

	err = provider2.Init(app)
	if err != nil {
		t.Fatalf("Failed to initialize provider2: %v", err)
	}

	// Try to inject service into consumer
	injectedModule, err := app.injectServices(consumer)
	if err != nil {
		t.Fatalf("Failed to inject services: %v", err)
	}

	// Verify that one of the services was injected
	injectedConsumer := injectedModule.(*InterfaceConsumerModule)
	if injectedConsumer.router == nil {
		t.Error("No router service was injected despite multiple matching services")
	}
}

// TestDependencyOrderWithInterfaceMatching tests that implicit dependencies
// from interface matching are properly reflected in module initialization order
func TestDependencyOrderWithInterfaceMatching(t *testing.T) {
	// Create a simple logger for testing
	testLogger := &logger{t}

	// Setup test application
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         testLogger,
	}

	// Register modules - consumer first, provider second (to test dependency resolution)
	consumer := &InterfaceConsumerModule{name: "consumer"}
	provider := &InterfaceProviderModule{name: "provider"}

	app.RegisterModule(consumer)
	app.RegisterModule(provider)

	// With the improved dependency resolution, the provider should come before consumer
	// even though we registered them in the opposite order
	order, err := app.resolveDependencies()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// Verify provider comes before consumer
	providerIdx := -1
	consumerIdx := -1
	for i, name := range order {
		switch name {
		case "provider":
			providerIdx = i
		case "consumer":
			consumerIdx = i
		}
	}

	if providerIdx == -1 || consumerIdx == -1 {
		t.Fatalf("Module order missing provider or consumer: %v", order)
	}

	if providerIdx > consumerIdx {
		t.Errorf("Expected provider to come before consumer, but got: %v", order)
	}
}

// HandleFuncService defines a router service interface
type HandleFuncService interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// InterfaceProviderModule simulates the chimux module that provides a router service
type InterfaceProviderModule struct {
	name string
}

func (m *InterfaceProviderModule) Name() string {
	return m.name
}

func (m *InterfaceProviderModule) Init(app Application) error {
	return nil
}

func (m *InterfaceProviderModule) Dependencies() []string {
	return nil
}

func (m *InterfaceProviderModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "router.service",
			Description: "Router service for testing",
			Instance:    m,
		},
	}
}

func (m *InterfaceProviderModule) RequiresServices() []ServiceDependency {
	return nil
}

// HandleFunc implements the HandleFuncService interface
func (m *InterfaceProviderModule) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	// Just a stub for testing
}

// InterfaceConsumerModule simulates the reverseproxy module that requires a router service
type InterfaceConsumerModule struct {
	name   string
	router handleFuncService
}

func (m *InterfaceConsumerModule) Name() string {
	return m.name
}

func (m *InterfaceConsumerModule) Init(app Application) error {
	return nil
}

func (m *InterfaceConsumerModule) Dependencies() []string {
	return nil
}

func (m *InterfaceConsumerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *InterfaceConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "router.service",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*handleFuncService)(nil)).Elem(),
		},
	}
}

func (m *InterfaceConsumerModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if router, ok := services["router.service"].(handleFuncService); ok {
			m.router = router
		}
		return m, nil
	}
}

// handleFuncService simulates the unexported interface in reverseproxy
type handleFuncService interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// IncompatibleProvider is a module that provides a service that does not implement HandleFuncService
type IncompatibleProvider struct {
	name string
}

func (m *IncompatibleProvider) Name() string {
	return m.name
}

func (m *IncompatibleProvider) Init(app Application) error {
	return nil
}

func (m *IncompatibleProvider) Dependencies() []string {
	return nil
}

func (m *IncompatibleProvider) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "router.service",
			Description: "Incompatible router service for testing",
			Instance:    m,
		},
	}
}

func (m *IncompatibleProvider) RequiresServices() []ServiceDependency {
	return nil
}

// NotHandleFunc is a method that does NOT match the HandleFuncService interface
func (m *IncompatibleProvider) NotHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	// Just a stub for testing
}

// CustomNameConsumerModule simulates a consumer module that expects a service with a custom name
type CustomNameConsumerModule struct {
	name   string
	router handleFuncService
}

func (m *CustomNameConsumerModule) Name() string {
	return m.name
}

func (m *CustomNameConsumerModule) Init(app Application) error {
	return nil
}

func (m *CustomNameConsumerModule) Dependencies() []string {
	return nil
}

func (m *CustomNameConsumerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *CustomNameConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*handleFuncService)(nil)).Elem(),
		},
	}
}

func (m *CustomNameConsumerModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if router, ok := services["router"].(handleFuncService); ok {
			m.router = router
		}
		return m, nil
	}
}
