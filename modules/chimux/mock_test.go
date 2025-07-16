package chimux

import (
	"context"
	"log/slog"
	"os"

	"github.com/CrisisTextLine/modular"
)

// MockLogger implements the modular.Logger interface for testing
type MockLogger struct{}

func (l *MockLogger) Debug(msg string, args ...interface{}) {}
func (l *MockLogger) Info(msg string, args ...interface{})  {}
func (l *MockLogger) Warn(msg string, args ...interface{})  {}
func (l *MockLogger) Error(msg string, args ...interface{}) {}

// MockApplication implements the modular.TenantApplication interface for testing
type MockApplication struct {
	configSections map[string]modular.ConfigProvider
	services       map[string]interface{}
	logger         modular.Logger
	tenantService  *MockTenantService
}

// NewMockApplication creates a new mock application for testing
func NewMockApplication() *MockApplication {
	tenantService := &MockTenantService{
		Configs: make(map[modular.TenantID]map[string]modular.ConfigProvider),
	}

	app := &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		services:       make(map[string]interface{}),
		logger:         &MockLogger{},
		tenantService:  tenantService,
	}

	// Register tenant service
	app.services["tenantService"] = tenantService

	return app
}

// ConfigProvider returns a simple ConfigProvider in the mock
func (m *MockApplication) ConfigProvider() modular.ConfigProvider {
	return NewStdConfigProvider(&mockAppConfig{})
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

	// Use type assertion for the most common types
	switch t := target.(type) {
	case *modular.TenantService:
		if svc, ok := service.(modular.TenantService); ok {
			*t = svc
			return nil
		}
	default:
		// For other types, try direct assignment
		val, ok := target.(*interface{})
		if ok {
			*val = service
		}
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

// Logger returns a mock logger
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

// TenantApplication interface methods
// GetTenantService returns the application's tenant service
func (m *MockApplication) GetTenantService() (modular.TenantService, error) {
	return m.tenantService, nil
}

// WithTenant creates a tenant context from the application context
func (m *MockApplication) WithTenant(tenantID modular.TenantID) (*modular.TenantContext, error) {
	return modular.NewTenantContext(context.Background(), tenantID), nil
}

// GetTenantConfig retrieves configuration for a specific tenant and section
func (m *MockApplication) GetTenantConfig(tenantID modular.TenantID, section string) (modular.ConfigProvider, error) {
	return m.tenantService.GetTenantConfig(tenantID, section)
}

// MockAppConfig is a simple configuration struct for testing
type mockAppConfig struct {
	Name    string
	Version string
}

// MockTenantService for testing tenant-related functionality
type MockTenantService struct {
	Configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (m *MockTenantService) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantSections, ok := m.Configs[tid]; ok {
		if provider, ok := tenantSections[section]; ok {
			return provider, nil
		}
	}
	// Return a default config provider for testing
	return NewStdConfigProvider(&ChiMuxConfig{}), nil
}

func (m *MockTenantService) GetTenants() []modular.TenantID {
	var tenants []modular.TenantID
	for tid := range m.Configs {
		tenants = append(tenants, tid)
	}
	return tenants
}

func (m *MockTenantService) RegisterTenant(tenantID modular.TenantID, configs map[string]modular.ConfigProvider) error {
	m.Configs[tenantID] = configs
	return nil
}

func (m *MockTenantService) RemoveTenant(tenantID modular.TenantID) error {
	delete(m.Configs, tenantID)
	return nil
}

func (m *MockTenantService) RegisterTenantAwareModule(module modular.TenantAwareModule) error {
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

// RealLogger returns a real slog logger for tests that need more detailed logging
func RealLogger() modular.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
