package reverseproxy

import (
	"context"
	"errors"
	"fmt"

	"github.com/GoCodeAlone/modular"
	"github.com/go-chi/chi/v5" // Import chi for router type assertion
)

var ErrMockConfigNotFound = errors.New("mock config not found for tenant")

// MockApplication implements the modular.Application interface for testing
type MockApplication struct {
	configSections map[string]modular.ConfigProvider
	services       map[string]interface{}
	logger         modular.Logger
}

// NewMockApplication creates a new mock application for testing
func NewMockApplication() *MockApplication {
	return &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		services:       make(map[string]interface{}),
		logger:         NewMockLogger(),
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
	service, exists := m.services[name]
	if !exists {
		return modular.ErrServiceNotFound
	}

	// Handle chi.Router specifically for our tests
	switch ptr := target.(type) {
	case *chi.Router:
		if router, ok := service.(chi.Router); ok {
			*ptr = router
			return nil
		}
	case *modular.TenantService:
		if tenantService, ok := service.(modular.TenantService); ok {
			*ptr = tenantService
			return nil
		}
	case *interface{}:
		*ptr = service
		return nil
	}

	// For other service types
	fmt.Printf("Warning: GetService called with unsupported target type for %s\n", name)
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

// Logger returns the mock logger
func (m *MockApplication) Logger() modular.Logger {
	return m.logger
}

// SetLogger sets the mock logger
func (m *MockApplication) SetLogger(logger modular.Logger) {
	m.logger = logger
}

// IsVerboseConfig returns whether verbose config is enabled (mock implementation)
func (m *MockApplication) IsVerboseConfig() bool {
	return false
}

// SetVerboseConfig sets the verbose config flag (mock implementation)
func (m *MockApplication) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

// Context returns a context for the mock application
func (m *MockApplication) Context() context.Context {
	return context.Background()
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

// MockTenantApplication implements modular.TenantApplication for testing
type MockTenantApplication struct {
	*MockApplication
	tenantService  *MockTenantService
	configProvider modular.ConfigProvider
}

// NewMockTenantApplication creates a new mock tenant application for testing
func NewMockTenantApplication() *MockTenantApplication {
	return &MockTenantApplication{
		MockApplication: NewMockApplication(),
		tenantService: &MockTenantService{
			Configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
		},
		configProvider: &mockConfigProvider{},
	}
}

// ConfigProvider returns the configProvider for interface compliance
func (m *MockTenantApplication) ConfigProvider() modular.ConfigProvider {
	return m.configProvider
}

// GetTenantConfig delegates to the tenant service
func (m *MockTenantApplication) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	return m.tenantService.GetTenantConfig(tid, section)
}

// GetTenants delegates to the tenant service
func (m *MockTenantApplication) GetTenants() []modular.TenantID {
	return m.tenantService.GetTenants()
}

// RegisterTenant delegates to the tenant service
func (m *MockTenantApplication) RegisterTenant(tid modular.TenantID, configs map[string]modular.ConfigProvider) error {
	return m.tenantService.RegisterTenant(tid, configs)
}

// RemoveTenant delegates to the tenant service
func (m *MockTenantApplication) RemoveTenant(tid modular.TenantID) error {
	return m.tenantService.RemoveTenant(tid)
}

// RegisterTenantAwareModule delegates to the tenant service
func (m *MockTenantApplication) RegisterTenantAwareModule(module modular.TenantAwareModule) error {
	return m.tenantService.RegisterTenantAwareModule(module)
}

// Add a Logger method to MockTenantApplication to ensure it correctly implements modular.TenantApplication
func (m *MockTenantApplication) Logger() modular.Logger {
	return m.MockApplication.Logger()
}

// SetLogger sets the logger by delegating to MockApplication
func (m *MockTenantApplication) SetLogger(logger modular.Logger) {
	m.MockApplication.SetLogger(logger)
}

// GetTenantService returns the tenant service
func (m *MockTenantApplication) GetTenantService() (modular.TenantService, error) {
	return m.tenantService, nil
}

// WithTenant returns a tenant context with the tenant ID
func (m *MockTenantApplication) WithTenant(tid modular.TenantID) (*modular.TenantContext, error) {
	// Return a simple tenant context for mock implementation
	tc := &modular.TenantContext{}
	// Use unexported field name as suggested by the error message
	// This is a workaround since we don't have the exact definition of modular.TenantContext
	return tc, nil
}

// MockTenantService ensures our mock fully implements modular.TenantApplication
type MockTenantService struct {
	Configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (m *MockTenantService) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantSections, ok := m.Configs[tid]; ok {
		if provider, ok := tenantSections[section]; ok {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("mock config not found for tenant %s, section %s: %w", tid, section, ErrMockConfigNotFound)
}

func (m *MockTenantService) GetTenants() []modular.TenantID {
	var tenants []modular.TenantID
	for tid := range m.Configs {
		tenants = append(tenants, tid)
	}
	return tenants
}

func (m *MockTenantService) RegisterTenant(tid modular.TenantID, configs map[string]modular.ConfigProvider) error {
	if m.Configs == nil {
		m.Configs = make(map[modular.TenantID]map[string]modular.ConfigProvider)
	}
	m.Configs[tid] = configs
	return nil
}

func (m *MockTenantService) RemoveTenant(tid modular.TenantID) error {
	delete(m.Configs, tid)
	return nil
}

func (m *MockTenantService) RegisterTenantAwareModule(module modular.TenantAwareModule) error {
	return nil
}

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	DebugMessages []string
	InfoMessages  []string
	WarnMessages  []string
	ErrorMessages []string
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		DebugMessages: make([]string, 0),
		InfoMessages:  make([]string, 0),
		WarnMessages:  make([]string, 0),
		ErrorMessages: make([]string, 0),
	}
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.DebugMessages = append(m.DebugMessages, fmt.Sprintf(msg, args...))
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.InfoMessages = append(m.InfoMessages, fmt.Sprintf(msg, args...))
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.WarnMessages = append(m.WarnMessages, fmt.Sprintf(msg, args...))
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.ErrorMessages = append(m.ErrorMessages, fmt.Sprintf(msg, args...))
}
