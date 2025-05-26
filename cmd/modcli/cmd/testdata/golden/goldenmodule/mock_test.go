package goldenmodule

import (
	"github.com/GoCodeAlone/modular"
)

// MockApplication implements the modular.Application interface for testing
type MockApplication struct {
	configSections map[string]modular.ConfigProvider
	services       map[string]interface{}
}

// NewMockApplication creates a new mock application for testing
func NewMockApplication() *MockApplication {
	return &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		services:       make(map[string]interface{}),
	}
}

// ConfigProvider returns a nil ConfigProvider in the mock
func (m *MockApplication) ConfigProvider() modular.ConfigProvider {
	return nil
}

// SvcRegistry returns the service registry
func (m *MockApplication) SvcRegistry() modular.ServiceRegistry {
	return m.services
}

// RegisterModule mocks module registration
func (m *MockApplication) RegisterModule(module modular.Module) {
	// No-op in mock
}

// RegisterConfigSection registers a config section with the mock app
func (m *MockApplication) RegisterConfigSection(section string, cp modular.ConfigProvider) {
	m.configSections[section] = cp
}

// ConfigSections returns all registered configuration sections
func (m *MockApplication) ConfigSections() map[string]modular.ConfigProvider {
	return m.configSections
}

// GetConfigSection retrieves a configuration section from the mock
func (m *MockApplication) GetConfigSection(section string) (modular.ConfigProvider, error) {
	cp, exists := m.configSections[section]
	if !exists {
		return nil, modular.ErrConfigSectionNotFound
	}
	return cp, nil
}

// RegisterService adds a service to the mock registry
func (m *MockApplication) RegisterService(name string, service interface{}) error {
	if _, exists := m.services[name]; exists {
		return modular.ErrServiceAlreadyRegistered
	}
	m.services[name] = service
	return nil
}

// GetService retrieves a service from the mock registry
func (m *MockApplication) GetService(name string, target interface{}) error {
	// Simple implementation that doesn't handle type conversion
	service, exists := m.services[name]
	if !exists {
		return modular.ErrServiceNotFound
	}

	// Just return the service without type checking for the mock
	// In a real implementation, this would properly handle the type conversion
	val, ok := target.(*interface{})
	if ok {
		*val = service
	}

	return nil
}

// Init mocks application initialization
func (m *MockApplication) Init() error {
	return nil
}

// Start mocks application start
func (m *MockApplication) Start() error {
	return nil
}

// Stop mocks application stop
func (m *MockApplication) Stop() error {
	return nil
}

// Run mocks application run
func (m *MockApplication) Run() error {
	return nil
}

// Logger returns a nil logger for the mock
func (m *MockApplication) Logger() modular.Logger {
	return nil
}

// NewStdConfigProvider is a simple mock implementation of modular.ConfigProvider
func NewStdConfigProvider(config interface{}) modular.ConfigProvider {
	return &mockConfigProvider{config: config}
}

type mockConfigProvider struct {
	config interface{}
}

func (m *mockConfigProvider) GetConfig() interface{} {
	return m.config
}
