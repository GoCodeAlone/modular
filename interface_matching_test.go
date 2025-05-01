package modular

import (
	"net/http"
	"reflect"
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
		if name == "provider" {
			providerIdx = i
		} else if name == "consumer" {
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
