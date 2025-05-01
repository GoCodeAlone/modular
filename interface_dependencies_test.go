package modular

import (
	"context"
	"reflect"
	"testing"
)

// TestInterfaceDependencies tests that modules with interface-based service dependencies
// are initialized in the correct order, even without explicit module dependencies.
func TestInterfaceDependencies(t *testing.T) {
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

	// Create and register modules
	serviceProviderModule := &RouterProviderModule{name: "router-provider"}
	serviceConsumerModule := &RouterConsumerModule{name: "router-consumer", t: t}

	// Register modules in reverse order to ensure dependency resolution works
	// (if it only used registration order, this would cause an issue)
	app.RegisterModule(serviceConsumerModule)
	app.RegisterModule(serviceProviderModule)

	// Resolve dependencies
	order, err := app.resolveDependencies()
	if err != nil {
		t.Fatalf("Failed to resolve dependencies: %v", err)
	}

	// Verify order - provider should be initialized before consumer
	providerFound := false
	consumerFound := false

	for _, moduleName := range order {
		if moduleName == "router-provider" {
			providerFound = true
		} else if moduleName == "router-consumer" {
			consumerFound = true
			// If we find the consumer before the provider, that's an error
			if !providerFound {
				t.Errorf("Consumer module ordered before provider module")
			}
		}
	}

	if !providerFound {
		t.Error("Provider module not found in initialization order")
	}
	if !consumerFound {
		t.Error("Consumer module not found in initialization order")
	}

	// Test full initialization to verify services are properly injected
	err = app.Init()
	if err != nil {
		t.Fatalf("App initialization failed: %v", err)
	}

	// Verify that consumer received the router service
	if !serviceConsumerModule.routerInjected {
		t.Error("Router service not injected into consumer module")
	}
}

// Router is a simple interface for testing service dependencies
type Router interface {
	HandleFunc(pattern string, handler func(string))
}

// SimpleRouter is an implementation of the Router interface
type SimpleRouter struct{}

func (r *SimpleRouter) HandleFunc(pattern string, handler func(string)) {
	// Just a stub implementation for testing
	handler(pattern)
}

// RouterProviderModule provides a Router service
type RouterProviderModule struct {
	name string
}

func (m *RouterProviderModule) Name() string {
	return m.name
}

func (m *RouterProviderModule) Init(app Application) error {
	return nil
}

func (m *RouterProviderModule) Dependencies() []string {
	// No explicit dependencies
	return nil
}

func (m *RouterProviderModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "router",
			Description: "Simple router service for testing",
			Instance:    &SimpleRouter{},
		},
	}
}

func (m *RouterProviderModule) RequiresServices() []ServiceDependency {
	return nil
}

// RouterConsumerModule requires a service implementing the Router interface
type RouterConsumerModule struct {
	name           string
	routerInjected bool
	t              *testing.T
}

func (m *RouterConsumerModule) Name() string {
	return m.name
}

func (m *RouterConsumerModule) Init(app Application) error {
	return nil
}

func (m *RouterConsumerModule) Dependencies() []string {
	// No explicit dependencies declared
	return nil
}

func (m *RouterConsumerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *RouterConsumerModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			MatchByInterface:   true,
			SatisfiesInterface: reflect.TypeOf((*Router)(nil)).Elem(),
		},
	}
}

func (m *RouterConsumerModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		if router, ok := services["router"].(Router); ok {
			// Mark that we received the router service
			m.routerInjected = true

			// Call router to verify it's working
			router.HandleFunc("/test", func(s string) {
				if s != "/test" {
					m.t.Errorf("Router received incorrect pattern: %s", s)
				}
			})
		}
		return m, nil
	}
}

func (m *RouterConsumerModule) Start(ctx context.Context) error {
	return nil
}

func (m *RouterConsumerModule) Stop(ctx context.Context) error {
	return nil
}
