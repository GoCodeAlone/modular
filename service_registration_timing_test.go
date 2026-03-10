package modular

import (
	"context"
	"testing"
)

// Test that services are available during Init() of dependent modules
func TestServiceRegistrationTiming(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(testCfg{Str: "test"}), &logger{t})

	// Create provider module that provides a service
	providerModule := &serviceProviderModule{
		name: "provider",
		services: []ServiceProvider{
			{
				Name:        "test.service",
				Description: "Test service",
				Instance:    "test-instance",
			},
		},
	}

	// Create consumer module that requires the service during Init
	// NOTE: This module uses Dependencies() to ensure proper initialization order
	consumerModule := &serviceConsumerModule{
		name:            "consumer",
		requiredService: "test.service",
		dependencies:    []string{"provider"}, // Depends on provider
	}

	// Register modules
	app.RegisterModule(providerModule)
	app.RegisterModule(consumerModule)

	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Expected Init to succeed, got error: %v", err)
	}

	// Verify that consumer module successfully received the service during Init
	if !consumerModule.serviceReceived {
		t.Error("Consumer module did not receive service during Init()")
	}

	if consumerModule.receivedValue != "test-instance" {
		t.Errorf("Consumer module received wrong service value: got %v, want %s", consumerModule.receivedValue, "test-instance")
	}
}

// Test that services are available via RequiresServices dependency injection with Constructor
func TestServiceRegistrationTimingWithConstructor(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(testCfg{Str: "test"}), &logger{t})

	// Create provider module that provides a service
	providerModule := &serviceProviderModule{
		name: "provider",
		services: []ServiceProvider{
			{
				Name:        "test.service",
				Description: "Test service",
				Instance:    "test-instance",
			},
		},
	}

	// Create consumer module that requires the service via RequiresServices
	// This should work even without explicit Dependencies()
	consumerModule := &serviceConsumerWithRequires{
		name: "consumer",
		requiredServices: []ServiceDependency{
			{
				Name:     "test.service",
				Required: true,
			},
		},
		dependencies: []string{"provider"}, // Still need this for ordering
	}

	// Register modules in order
	app.RegisterModule(providerModule)
	app.RegisterModule(consumerModule)

	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Expected Init to succeed, got error: %v", err)
	}

	// Verify that consumer module received injected services
	if !consumerModule.servicesInjected {
		t.Error("Consumer module did not receive injected services")
	}
}

// serviceProviderModule is a test module that provides a service
type serviceProviderModule struct {
	testModule
	name     string
	services []ServiceProvider
}

func (m *serviceProviderModule) Name() string {
	return m.name
}

func (m *serviceProviderModule) Init(app Application) error {
	return nil
}

func (m *serviceProviderModule) ProvidesServices() []ServiceProvider {
	return m.services
}

func (m *serviceProviderModule) RequiresServices() []ServiceDependency {
	return nil
}

// serviceConsumerModule is a test module that requires a service during Init
type serviceConsumerModule struct {
	testModule
	name            string
	requiredService string
	dependencies    []string
	serviceReceived bool
	receivedValue   any
}

func (m *serviceConsumerModule) Name() string {
	return m.name
}

func (m *serviceConsumerModule) Dependencies() []string {
	return m.dependencies
}

func (m *serviceConsumerModule) Init(app Application) error {
	// Try to get the required service during Init
	var service any
	err := app.GetService(m.requiredService, &service)
	if err != nil {
		return err
	}

	m.serviceReceived = true
	m.receivedValue = service
	return nil
}

func (m *serviceConsumerModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *serviceConsumerModule) RequiresServices() []ServiceDependency {
	return nil
}

// Test with actual scheduler-like scenario
func TestSchedulerServiceRegistrationTiming(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(testCfg{Str: "test"}), &logger{t})

	// Create a scheduler-like provider module
	schedulerModule := &schedulerLikeModule{
		name: "scheduler",
	}

	// Create a jobs-like consumer module
	jobsModule := &jobsLikeModule{
		name:         "jobs",
		dependencies: []string{"scheduler"},
	}

	// Register modules
	app.RegisterModule(schedulerModule)
	app.RegisterModule(jobsModule)

	// Initialize application
	err := app.Init()
	if err != nil {
		t.Fatalf("Expected Init to succeed, got error: %v", err)
	}

	// Verify that jobs module successfully registered jobs with scheduler
	if !jobsModule.jobsRegistered {
		t.Error("Jobs module failed to register jobs with scheduler during Init()")
	}
}

// schedulerLikeModule mimics the scheduler module behavior
type schedulerLikeModule struct {
	testModule
	name          string
	registeredJob string
}

func (m *schedulerLikeModule) Name() string {
	return m.name
}

func (m *schedulerLikeModule) Init(app Application) error {
	// Scheduler initializes its internal state
	return nil
}

func (m *schedulerLikeModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{
			Name:        "scheduler.provider",
			Description: "Job scheduling service",
			Instance:    m,
		},
	}
}

func (m *schedulerLikeModule) RequiresServices() []ServiceDependency {
	return nil
}

func (m *schedulerLikeModule) RegisterJob(name string) {
	m.registeredJob = name
}

// jobsLikeModule mimics a module that depends on scheduler
type jobsLikeModule struct {
	testModule
	name           string
	dependencies   []string
	jobsRegistered bool
}

func (m *jobsLikeModule) Name() string {
	return m.name
}

func (m *jobsLikeModule) Dependencies() []string {
	return m.dependencies
}

func (m *jobsLikeModule) Init(app Application) error {
	// Try to get scheduler service during Init
	var scheduler *schedulerLikeModule
	err := app.GetService("scheduler.provider", &scheduler)
	if err != nil {
		return err
	}

	// Register jobs with scheduler
	scheduler.RegisterJob("test-job")
	m.jobsRegistered = true
	return nil
}

func (m *jobsLikeModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *jobsLikeModule) RequiresServices() []ServiceDependency {
	return nil
}

func (m *jobsLikeModule) Start(ctx context.Context) error {
	return nil
}

// serviceConsumerWithRequires is a test module that uses RequiresServices for dependency injection
type serviceConsumerWithRequires struct {
	testModule
	name             string
	requiredServices []ServiceDependency
	dependencies     []string
	servicesInjected bool
	injectedService  any
}

func (m *serviceConsumerWithRequires) Name() string {
	return m.name
}

func (m *serviceConsumerWithRequires) Dependencies() []string {
	return m.dependencies
}

func (m *serviceConsumerWithRequires) Init(app Application) error {
	// Init is called after service injection via Constructor
	return nil
}

func (m *serviceConsumerWithRequires) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *serviceConsumerWithRequires) RequiresServices() []ServiceDependency {
	return m.requiredServices
}

func (m *serviceConsumerWithRequires) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		// This should be called with the injected services
		if len(services) > 0 {
			m.servicesInjected = true
			if svc, ok := services["test.service"]; ok {
				m.injectedService = svc
			}
		}
		return m, nil
	}
}

// serviceConsumerWithDeclaredRequires uses RequiresServices to declare dependencies
// and also accesses the service during Init via GetService
type serviceConsumerWithDeclaredRequires struct {
	testModule
	name             string
	requiredServices []ServiceDependency
	requiredService  string
	serviceReceived  bool
	receivedValue    any
}

func (m *serviceConsumerWithDeclaredRequires) Name() string {
	return m.name
}

func (m *serviceConsumerWithDeclaredRequires) Init(app Application) error {
	// Try to get the required service during Init
	var service any
	err := app.GetService(m.requiredService, &service)
	if err != nil {
		return err
	}

	m.serviceReceived = true
	m.receivedValue = service
	return nil
}

func (m *serviceConsumerWithDeclaredRequires) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *serviceConsumerWithDeclaredRequires) RequiresServices() []ServiceDependency {
	return m.requiredServices
}

// Test service registration timing without explicit Dependencies() - demonstrates the failure scenario
// When a module accesses services via GetService() during Init() but doesn't declare
// RequiresServices(), the initialization order may be wrong
func TestServiceRegistrationTimingWithoutDependencies(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(testCfg{Str: "test"}), &logger{t})

	// Create provider module that provides a service
	providerModule := &serviceProviderModule{
		name: "provider",
		services: []ServiceProvider{
			{
				Name:        "test.service",
				Description: "Test service",
				Instance:    "test-instance",
			},
		},
	}

	// Create consumer module that requires the service during Init
	// NOTE: This module does NOT use Dependencies() or RequiresServices()
	// This demonstrates the failure scenario described in the issue.
	consumerModule := &serviceConsumerModule{
		name:            "consumer",
		requiredService: "test.service",
		// NO dependencies declaration - this is the bug scenario!
	}

	// Register modules - order matters without Dependencies()
	app.RegisterModule(providerModule)
	app.RegisterModule(consumerModule)

	// Initialize application - this currently fails because of alphabetical ordering
	err := app.Init()

	// This test documents the current behavior: it fails
	if err == nil {
		t.Error("Expected Init to fail due to missing service (alphabetical ordering), but it succeeded")
	}
}

// Test that RequiresServices properly creates implicit dependencies
func TestServiceRegistrationTimingWithRequiresServices(t *testing.T) {
	app := NewStdApplication(NewStdConfigProvider(testCfg{Str: "test"}), &logger{t})

	// Create provider module that provides a service
	providerModule := &serviceProviderModule{
		name: "provider",
		services: []ServiceProvider{
			{
				Name:        "test.service",
				Description: "Test service",
				Instance:    "test-instance",
			},
		},
	}

	// Create consumer module that properly declares RequiresServices()
	consumerModule := &serviceConsumerWithDeclaredRequires{
		name: "consumer",
		requiredServices: []ServiceDependency{
			{
				Name:     "test.service",
				Required: true,
			},
		},
		requiredService: "test.service",
	}

	// Register modules - order doesn't matter with RequiresServices()
	app.RegisterModule(consumerModule) // Register consumer first!
	app.RegisterModule(providerModule)

	// Initialize application - should succeed because RequiresServices creates implicit dependency
	err := app.Init()
	if err != nil {
		t.Fatalf("Expected Init to succeed with RequiresServices(), got error: %v", err)
	}

	// Verify that consumer module successfully received the service during Init
	if !consumerModule.serviceReceived {
		t.Error("Consumer module did not receive service during Init()")
	}
}
