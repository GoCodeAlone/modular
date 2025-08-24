package auth

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockApplication is a mock implementation of modular.Application for testing
type MockApplication struct {
	configSections map[string]modular.ConfigProvider
	services       map[string]interface{}
	logger         modular.Logger
}

// NewMockApplication creates a new mock application
func NewMockApplication() *MockApplication {
	return &MockApplication{
		configSections: make(map[string]modular.ConfigProvider),
		services:       make(map[string]interface{}),
		logger:         MockLogger{},
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
	if service, exists := m.services[name]; exists {
		// In a real implementation, this would use reflection to set the target
		_ = service
		_ = target
		return nil
	}
	return ErrUserNotFound // Using existing error for simplicity
}

// Logger returns the mock logger
func (m *MockApplication) Logger() modular.Logger {
	return m.logger
}

// SetLogger sets the mock logger
func (m *MockApplication) SetLogger(logger modular.Logger) {
	m.logger = logger
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

// IsVerboseConfig returns whether verbose config is enabled (mock implementation)
func (m *MockApplication) IsVerboseConfig() bool {
	return false
}

// SetVerboseConfig sets the verbose config flag (mock implementation)
func (m *MockApplication) SetVerboseConfig(verbose bool) {
	// No-op in mock
}

// MockLogger implements a minimal logger for testing
type MockLogger struct{}

func (m MockLogger) Info(msg string, args ...interface{})  {}
func (m MockLogger) Error(msg string, args ...interface{}) {}
func (m MockLogger) Debug(msg string, args ...interface{}) {}
func (m MockLogger) Warn(msg string, args ...interface{})  {}

func TestModule_NewModule(t *testing.T) {
	module := NewModule()
	assert.NotNil(t, module)
	assert.Equal(t, "auth", module.Name())
}

func TestModule_Dependencies(t *testing.T) {
	module := &Module{}
	deps := module.Dependencies()
	assert.Empty(t, deps, "Auth module should have no explicit module dependencies")
}

func TestModule_ProvidesServices(t *testing.T) {
	module := &Module{
		service: &Service{}, // Mock service
	}

	services := module.ProvidesServices()
	require.Len(t, services, 1)

	assert.Equal(t, ServiceName, services[0].Name)
	assert.Equal(t, "Authentication service providing JWT, sessions, and OAuth2 support", services[0].Description)
	assert.NotNil(t, services[0].Instance)
}

func TestModule_RequiresServices(t *testing.T) {
	module := &Module{}
	deps := module.RequiresServices()

	require.Len(t, deps, 2)

	// Check user_store dependency
	userStoreDep := deps[0]
	assert.Equal(t, "user_store", userStoreDep.Name)
	assert.False(t, userStoreDep.Required, "user_store should be optional")

	// Check session_store dependency
	sessionStoreDep := deps[1]
	assert.Equal(t, "session_store", sessionStoreDep.Name)
	assert.False(t, sessionStoreDep.Required, "session_store should be optional")
}

func TestModule_RegisterConfig(t *testing.T) {
	module := &Module{}
	app := NewMockApplication()

	err := module.RegisterConfig(app)
	assert.NoError(t, err)
	assert.NotNil(t, module.config)

	// Verify config was registered with the app
	sections := app.ConfigSections()
	_, exists := sections["auth"]
	assert.True(t, exists)
}

func TestModule_Init(t *testing.T) {
	// Test with valid config
	module := &Module{}

	config := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret",
			Expiration:        3600,
			RefreshExpiration: 86400,
		},
		Password: PasswordConfig{
			MinLength:  8,
			BcryptCost: 12,
		},
	}

	app := NewMockApplication()
	// Register the config section
	app.RegisterConfigSection("auth", modular.NewStdConfigProvider(config))

	err := module.Init(app)
	assert.NoError(t, err)
	assert.NotNil(t, module.logger)
	assert.NotNil(t, module.config)
}

func TestModule_Init_InvalidConfig(t *testing.T) {
	// Test with invalid config
	module := &Module{}

	config := &Config{
		JWT: JWTConfig{
			Secret: "", // Invalid: empty secret
		},
	}

	app := NewMockApplication()
	// Register the invalid config section
	app.RegisterConfigSection("auth", modular.NewStdConfigProvider(config))

	err := module.Init(app)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
}

func TestModule_StartStop(t *testing.T) {
	module := &Module{
		logger: MockLogger{},
	}

	ctx := context.Background()

	// Test Start
	err := module.Start(ctx)
	assert.NoError(t, err)

	// Test Stop
	err = module.Stop(ctx)
	assert.NoError(t, err)
}

func TestModule_Constructor(t *testing.T) {
	module := &Module{
		config: &Config{
			JWT: JWTConfig{
				Secret:            "test-secret",
				Expiration:        3600,
				RefreshExpiration: 86400,
			},
		},
	}

	app := NewMockApplication()
	constructor := module.Constructor()

	// Test constructor with no services (should use defaults)
	services := make(map[string]any)
	resultModule, err := constructor(app, services)
	require.NoError(t, err)
	require.NotNil(t, resultModule)

	authModule, ok := resultModule.(*Module)
	require.True(t, ok)
	assert.NotNil(t, authModule.service)
}

func TestModule_Constructor_WithCustomStores(t *testing.T) {
	module := &Module{
		config: &Config{
			JWT: JWTConfig{
				Secret:            "test-secret",
				Expiration:        3600,
				RefreshExpiration: 86400,
			},
		},
	}

	app := NewMockApplication()
	constructor := module.Constructor()

	// Test constructor with custom stores
	userStore := NewMemoryUserStore()
	sessionStore := NewMemorySessionStore()

	services := map[string]any{
		"user_store":    userStore,
		"session_store": sessionStore,
	}

	resultModule, err := constructor(app, services)
	require.NoError(t, err)
	require.NotNil(t, resultModule)

	authModule, ok := resultModule.(*Module)
	require.True(t, ok)
	assert.NotNil(t, authModule.service)
}

func TestModule_Constructor_InvalidUserStore(t *testing.T) {
	module := &Module{
		config: &Config{
			JWT: JWTConfig{
				Secret:            "test-secret",
				Expiration:        3600,
				RefreshExpiration: 86400,
			},
		},
	}

	app := NewMockApplication()
	constructor := module.Constructor()

	// Test constructor with invalid user store
	services := map[string]any{
		"user_store": "invalid-store", // Not a UserStore implementation
	}

	_, err := constructor(app, services)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user_store service does not implement UserStore interface")
}

func TestModule_Constructor_InvalidSessionStore(t *testing.T) {
	module := &Module{
		config: &Config{
			JWT: JWTConfig{
				Secret:            "test-secret",
				Expiration:        3600,
				RefreshExpiration: 86400,
			},
		},
	}

	app := NewMockApplication()
	constructor := module.Constructor()

	// Test constructor with invalid session store
	services := map[string]any{
		"session_store": "invalid-store", // Not a SessionStore implementation
	}

	_, err := constructor(app, services)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session_store service does not implement SessionStore interface")
}
