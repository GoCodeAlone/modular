package goldenmodule

import (
	// "context" // Removed, Stop() doesn't need it
	"fmt" // Import fmt for error formatting
	"github.com/GoCodeAlone/modular"
	// "log/slog" // Removed, Logger returns nil
	// "os"       // Removed, Logger returns nil
)

// MockApplication is a mock implementation of the modular.Application interface for testing
type MockApplication struct {
	registeredConfigSections map[string]modular.ConfigProvider
	registeredServices       map[string]interface{}
}

// MockConfigProvider is a minimal implementation for testing
type MockConfigProvider struct{}
func (m *MockConfigProvider) Load() error { return nil }
func (m *MockConfigProvider) Get(key string) interface{} { return nil }
func (m *MockConfigProvider) Unmarshal(target interface{}) error { return nil }
func (m *MockConfigProvider) UnmarshalKey(key string, target interface{}) error { return nil }
func (m *MockConfigProvider) AllSettings() map[string]interface{} { return nil }
func (m *MockConfigProvider) SetDefault(key string, value interface{}) {}
func (m *MockConfigProvider) GetConfig() interface{} { return nil }


func NewMockApplication() *MockApplication {
	return &MockApplication{
		registeredConfigSections: make(map[string]modular.ConfigProvider),
		registeredServices:       make(map[string]interface{}),
	}
}

// Init initializes the mock application
func (m *MockApplication) Init() error {
	// No-op for mock
	return nil
}

// Run executes the mock application lifecycle
func (m *MockApplication) Run() error {
	// No-op for mock, just return nil
	return nil
}

// Start begins the application startup process
func (m *MockApplication) Start() error {
	// No-op for mock
	return nil
}

// Stop ends the application lifecycle (Corrected signature)
func (m *MockApplication) Stop() error {
	// No-op for mock
	return nil
}


func (m *MockApplication) RegisterModule(module modular.Module) {
	// No-op for tests
}

// RegisterService stores a service instance
func (m *MockApplication) RegisterService(name string, service interface{}) error {
	if m.registeredServices == nil {
		m.registeredServices = make(map[string]interface{})
	}
	m.registeredServices[name] = service
	return nil
}

// GetService retrieves a service instance (Re-added)
func (m *MockApplication) GetService(name string, target interface{}) error {
	service, ok := m.registeredServices[name]
	if !ok {
		return fmt.Errorf("service '%s' not found", name)
	}
	// Basic type assertion/copy for mock - real implementation might use reflection
	if svcPtr, ok := target.(*interface{}); ok {
		*svcPtr = service
		return nil
	}
	// Add more specific type checks if needed for your tests
	return fmt.Errorf("target for GetService must be a pointer to an interface{} or the correct type")
}

// SvcRegistry returns the service registry (added)
func (m *MockApplication) SvcRegistry() modular.ServiceRegistry {
	return m.registeredServices
}


func (m *MockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.registeredConfigSections[name] = provider
}

// GetConfigSection retrieves a registered config provider by name
func (m *MockApplication) GetConfigSection(name string) (modular.ConfigProvider, error) {
	provider, ok := m.registeredConfigSections[name]
	if !ok {
		return nil, fmt.Errorf("config section '%s' not found", name)
	}
	return provider, nil
}


func (m *MockApplication) Logger() modular.Logger {
	// Return nil for simplicity
	return nil
}

// ConfigProvider returns the main configuration provider
func (m *MockApplication) ConfigProvider() modular.ConfigProvider {
	// Return a simple mock provider
	return &MockConfigProvider{}
}

// ConfigSections returns the map of registered config providers
func (m *MockApplication) ConfigSections() map[string]modular.ConfigProvider {
	return m.registeredConfigSections
}
