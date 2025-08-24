// Package reverseproxy provides a reverse proxy module for the modular framework
package reverseproxy

import (
	"fmt"
	"net/http"

	"github.com/CrisisTextLine/modular"
	"github.com/stretchr/testify/mock"
)

// MockConfigProvider implements a mock configuration provider using testify/mock
type MockConfigProvider struct {
	mock.Mock
}

// GetConfig returns the mocked configuration
func (m *MockConfigProvider) GetConfig() interface{} {
	args := m.Called()
	return args.Get(0)
}

// MockRouter implements a mock router using testify/mock
type MockRouter struct {
	mock.Mock
}

// Handle mocks the Handle method of a router
func (m *MockRouter) Handle(pattern string, handler http.Handler) {
	m.Called(pattern, handler)
}

// HandleFunc mocks the HandleFunc method of a router
func (m *MockRouter) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.Called(pattern, handler)
}

// Mount mocks the Mount method of a router
func (m *MockRouter) Mount(pattern string, h http.Handler) {
	m.Called(pattern, h)
}

// Use mocks the Use method of a router
func (m *MockRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	m.Called(middlewares)
}

// ServeHTTP mocks the ServeHTTP method of a router
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

// MockTenantApplicationWithMock embeds MockTenantApplication with testify/mock functionality
type MockTenantApplicationWithMock struct {
	*MockTenantApplication
	mock.Mock
}

// NewMockTenantApplicationWithMock creates a new mock tenant application with testify/mock functionality
func NewMockTenantApplicationWithMock() *MockTenantApplicationWithMock {
	return &MockTenantApplicationWithMock{
		MockTenantApplication: NewMockTenantApplication(),
	}
}

// GetConfigSection retrieves a configuration section from the mock with testify/mock support
func (m *MockTenantApplicationWithMock) GetConfigSection(section string) (modular.ConfigProvider, error) {
	args := m.Called(section)
	if err := args.Error(1); err != nil {
		return args.Get(0).(modular.ConfigProvider), fmt.Errorf("mock GetConfigSection error: %w", err)
	}
	return args.Get(0).(modular.ConfigProvider), nil
}

// GetTenantConfig retrieves tenant-specific configuration with testify/mock support
func (m *MockTenantApplicationWithMock) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	args := m.Called(tid, section)
	if err := args.Error(1); err != nil {
		return args.Get(0).(modular.ConfigProvider), fmt.Errorf("mock GetTenantConfig error: %w", err)
	}
	return args.Get(0).(modular.ConfigProvider), nil
}
