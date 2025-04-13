package goldenmodule

import (
	"github.com/GoCodeAlone/modular"
)

// MockApplication is a mock implementation of the modular.Application interface for testing
type MockApplication struct {
	ConfigSections map[string]modular.ConfigProvider
}

func NewMockApplication() *MockApplication {
	return &MockApplication{
		ConfigSections: make(map[string]modular.ConfigProvider),
	}
}

func (m *MockApplication) RegisterModule(module modular.Module) {
	// No-op for tests
}

func (m *MockApplication) RegisterService(name string, service interface{}) error {
	return nil
}

func (m *MockApplication) GetService(name string, target interface{}) error {
	return nil
}

func (m *MockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.ConfigSections[name] = provider
}

func (m *MockApplication) Logger() modular.Logger {
	return nil
}
