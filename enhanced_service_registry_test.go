package modular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test interfaces and implementations
type ServiceRegistryTestInterface interface {
	TestMethod() string
}

type ServiceRegistryTestImplementation1 struct{}

func (t *ServiceRegistryTestImplementation1) TestMethod() string { return "impl1" }

type ServiceRegistryTestImplementation2 struct{}

func (t *ServiceRegistryTestImplementation2) TestMethod() string { return "impl2" }

type ServiceRegistryTestModule1 struct{}

func (m *ServiceRegistryTestModule1) Name() string               { return "module1" }
func (m *ServiceRegistryTestModule1) Init(app Application) error { return nil }

type ServiceRegistryTestModule2 struct{}

func (m *ServiceRegistryTestModule2) Name() string               { return "module2" }
func (m *ServiceRegistryTestModule2) Init(app Application) error { return nil }

func TestEnhancedServiceRegistry_BasicRegistration(t *testing.T) {
	registry := NewEnhancedServiceRegistry()

	service := &ServiceRegistryTestImplementation1{}
	actualName, err := registry.RegisterService("testService", service)

	require.NoError(t, err)
	assert.Equal(t, "testService", actualName)

	// Verify service can be retrieved
	retrieved, found := registry.GetService("testService")
	assert.True(t, found)
	assert.Equal(t, service, retrieved)
}

func TestEnhancedServiceRegistry_ModuleTracking(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	module := &ServiceRegistryTestModule1{}

	registry.SetCurrentModule(module)
	service := &ServiceRegistryTestImplementation1{}
	actualName, err := registry.RegisterService("testService", service)
	registry.ClearCurrentModule()

	require.NoError(t, err)
	assert.Equal(t, "testService", actualName)

	// Verify module association
	entry, found := registry.GetServiceEntry("testService")
	assert.True(t, found)
	assert.Equal(t, "module1", entry.ModuleName)
	assert.Equal(t, reflect.TypeOf(module), entry.ModuleType)
	assert.Equal(t, "testService", entry.OriginalName)
	assert.Equal(t, "testService", entry.ActualName)
}

func TestEnhancedServiceRegistry_NameConflictResolution(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	module1 := &ServiceRegistryTestModule1{}
	module2 := &ServiceRegistryTestModule2{}

	// Register first service
	registry.SetCurrentModule(module1)
	service1 := &ServiceRegistryTestImplementation1{}
	actualName1, err := registry.RegisterService("service", service1)
	registry.ClearCurrentModule()

	require.NoError(t, err)
	assert.Equal(t, "service", actualName1) // First one gets original name

	// Register second service with same name
	registry.SetCurrentModule(module2)
	service2 := &ServiceRegistryTestImplementation2{}
	actualName2, err := registry.RegisterService("service", service2)
	registry.ClearCurrentModule()

	require.NoError(t, err)
	assert.Equal(t, "service.module2", actualName2) // Second one gets module-suffixed name

	// Verify both services are retrievable
	retrieved1, found := registry.GetService("service")
	assert.True(t, found)
	assert.Equal(t, service1, retrieved1)

	retrieved2, found := registry.GetService("service.module2")
	assert.True(t, found)
	assert.Equal(t, service2, retrieved2)

	// Verify module associations
	services1 := registry.GetServicesByModule("module1")
	assert.Equal(t, []string{"service"}, services1)

	services2 := registry.GetServicesByModule("module2")
	assert.Equal(t, []string{"service.module2"}, services2)
}

func TestEnhancedServiceRegistry_InterfaceDiscovery(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	module1 := &ServiceRegistryTestModule1{}
	module2 := &ServiceRegistryTestModule2{}

	// Register services implementing ServiceRegistryTestInterface
	registry.SetCurrentModule(module1)
	service1 := &ServiceRegistryTestImplementation1{}
	registry.RegisterService("impl1", service1)
	registry.ClearCurrentModule()

	registry.SetCurrentModule(module2)
	service2 := &ServiceRegistryTestImplementation2{}
	registry.RegisterService("impl2", service2)
	registry.ClearCurrentModule()

	// Register a service that doesn't implement the interface
	nonInterfaceService := "not an interface"
	registry.RegisterService("nonInterface", nonInterfaceService)

	// Discover by interface
	interfaceType := reflect.TypeOf((*ServiceRegistryTestInterface)(nil)).Elem()
	entries := registry.GetServicesByInterface(interfaceType)

	require.Len(t, entries, 2)

	// Sort by service name for consistent testing
	if entries[0].ActualName > entries[1].ActualName {
		entries[0], entries[1] = entries[1], entries[0]
	}

	assert.Equal(t, "impl1", entries[0].ActualName)
	assert.Equal(t, "module1", entries[0].ModuleName)
	assert.Equal(t, service1, entries[0].Service)

	assert.Equal(t, "impl2", entries[1].ActualName)
	assert.Equal(t, "module2", entries[1].ModuleName)
	assert.Equal(t, service2, entries[1].Service)
}

func TestEnhancedServiceRegistry_BackwardsCompatibility(t *testing.T) {
	registry := NewEnhancedServiceRegistry()

	service1 := &ServiceRegistryTestImplementation1{}
	service2 := &ServiceRegistryTestImplementation2{}

	registry.RegisterService("service1", service1)
	registry.RegisterService("service2", service2)

	// Test backwards compatible view
	compatRegistry := registry.AsServiceRegistry()

	assert.Equal(t, service1, compatRegistry["service1"])
	assert.Equal(t, service2, compatRegistry["service2"])
	assert.Len(t, compatRegistry, 2)
}

func TestEnhancedServiceRegistry_ComplexConflictResolution(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	module := &ServiceRegistryTestModule1{}

	registry.SetCurrentModule(module)

	// Register multiple services with same name from same module
	service1 := &ServiceRegistryTestImplementation1{}
	actualName1, err := registry.RegisterService("service", service1)
	require.NoError(t, err)
	assert.Equal(t, "service", actualName1)

	service2 := &ServiceRegistryTestImplementation2{}
	actualName2, err := registry.RegisterService("service", service2)
	require.NoError(t, err)
	assert.Equal(t, "service.module1", actualName2) // First fallback: module name

	service3 := "third service"
	actualName3, err := registry.RegisterService("service", service3)
	require.NoError(t, err)
	// Second fallback tries module type name first, then counter
	expectedName3 := "service.ServiceRegistryTestModule1"
	assert.Equal(t, expectedName3, actualName3) // Module type name fallback

	registry.ClearCurrentModule()

	// Verify all services are accessible
	retrieved1, found := registry.GetService("service")
	assert.True(t, found)
	assert.Equal(t, service1, retrieved1)

	retrieved2, found := registry.GetService("service.module1")
	assert.True(t, found)
	assert.Equal(t, service2, retrieved2)

	retrieved3, found := registry.GetService(expectedName3)
	assert.True(t, found)
	assert.Equal(t, service3, retrieved3)
}

func TestEnhancedServiceRegistry_CounterFallback(t *testing.T) {
	registry := NewEnhancedServiceRegistry()
	module := &ServiceRegistryTestModule1{}

	registry.SetCurrentModule(module)

	// Register services that exhaust module name and type name options
	service1 := &ServiceRegistryTestImplementation1{}
	actualName1, err := registry.RegisterService("service", service1)
	require.NoError(t, err)
	assert.Equal(t, "service", actualName1)

	service2 := &ServiceRegistryTestImplementation2{}
	actualName2, err := registry.RegisterService("service", service2)
	require.NoError(t, err)
	assert.Equal(t, "service.module1", actualName2)

	service3 := "third service"
	actualName3, err := registry.RegisterService("service", service3)
	require.NoError(t, err)
	assert.Equal(t, "service.ServiceRegistryTestModule1", actualName3)

	// Force another registration that will use counter fallback
	// by registering a service that conflicts with the module type name too
	service4 := "fourth service"
	_, err = registry.RegisterService("service.ServiceRegistryTestModule1", service4)
	require.NoError(t, err)

	// Now the fifth service should use counter fallback
	service5 := "fifth service"
	actualName5, err := registry.RegisterService("service", service5)
	require.NoError(t, err)
	assert.Equal(t, "service.2", actualName5) // Counter reflects attempts at original name

	registry.ClearCurrentModule()

	// Verify service is accessible
	retrieved5, found := registry.GetService("service.2")
	assert.True(t, found)
	assert.Equal(t, service5, retrieved5)
}
