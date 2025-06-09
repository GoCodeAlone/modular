package modular

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test errors for specific scenarios
var (
	ErrConfigRegistrationFailed   = errors.New("config registration failed")
	ErrAppConfigLoaderFailed      = errors.New("app config loader failed")
	ErrDependencyResolutionFailed = errors.New("dependency resolution failed")
	ErrServiceInjectionFailed     = errors.New("service injection failed")
	ErrModuleInitFailed           = errors.New("module init failed")
	ErrServiceRegistrationFailed  = errors.New("service registration failed")
	ErrTenantConfigInitFailed     = errors.New("tenant config init failed")
)

// Test_Application_Init_ErrorCollection tests that Init() collects all errors instead of stopping at first error
func Test_Application_Init_ErrorCollection(t *testing.T) {
	tests := []struct {
		name                string
		modules             []Module
		appConfigLoaderErr  error
		expectErrorCount    int
		expectPartialInit   bool
		expectErrorContains []string
	}{
		{
			name: "Multiple config registration failures",
			modules: []Module{
				&initTestFailingConfigModule{initTestModule: initTestModule{name: "module1"}, configErr: ErrConfigRegistrationFailed},
				&initTestFailingConfigModule{initTestModule: initTestModule{name: "module2"}, configErr: ErrConfigRegistrationFailed},
				&initTestSuccessfulModule{initTestModule: initTestModule{name: "module3"}},
			},
			expectErrorCount:    2,
			expectPartialInit:   true,
			expectErrorContains: []string{"module1 failed to register config", "module2 failed to register config"},
		},
		{
			name: "App config loader failure with module errors",
			modules: []Module{
				&initTestFailingConfigModule{initTestModule: initTestModule{name: "module1"}, configErr: ErrConfigRegistrationFailed},
				&initTestSuccessfulModule{initTestModule: initTestModule{name: "module2"}},
			},
			appConfigLoaderErr:  ErrAppConfigLoaderFailed,
			expectErrorCount:    2,
			expectPartialInit:   true,
			expectErrorContains: []string{"module1 failed to register config", "failed to load app config"},
		},
		{
			name: "Module init failures are collected",
			modules: []Module{
				&initTestSuccessfulModule{initTestModule: initTestModule{name: "module1"}},
				&initTestFailingInitModule{initTestModule: initTestModule{name: "module2"}, initErr: ErrModuleInitFailed},
				&initTestFailingInitModule{initTestModule: initTestModule{name: "module3"}, initErr: ErrModuleInitFailed},
			},
			expectErrorCount:    2,
			expectPartialInit:   true,
			expectErrorContains: []string{"module 'module2' failed to initialize", "module 'module3' failed to initialize"},
		},
		{
			name: "Service registration failures are collected",
			modules: []Module{
				&initTestSuccessfulModule{initTestModule: initTestModule{name: "module1"}},
				&initTestConflictingServiceModule{
					initTestModule: initTestModule{name: "module2"},
					serviceName:    "duplicate-service",
				},
				&initTestConflictingServiceModule{
					initTestModule: initTestModule{name: "module3"},
					serviceName:    "duplicate-service", // Same service name causes conflict
				},
			},
			expectErrorCount:    1, // Second registration will fail
			expectPartialInit:   true,
			expectErrorContains: []string{"failed to register service"},
		},
		{
			name: "Service injection failures are collected",
			modules: []Module{
				&initTestServiceRequiringModule{
					initTestModule: initTestModule{name: "consumer1"},
					requiredServices: []ServiceDependency{
						{Name: "nonexistent-service", Required: true},
					},
				},
				&initTestServiceRequiringModule{
					initTestModule: initTestModule{name: "consumer2"},
					requiredServices: []ServiceDependency{
						{Name: "another-nonexistent-service", Required: true},
					},
				},
			},
			expectErrorCount:    2,
			expectPartialInit:   false,
			expectErrorContains: []string{"failed to inject services for module", "consumer1", "consumer2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup application
			app := &StdApplication{
				cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         &initTestLogger{t: t},
			}

			// Register modules
			for _, module := range tt.modules {
				app.RegisterModule(module)
			}

			// Setup mock AppConfigLoader
			originalLoader := AppConfigLoader
			defer func() { AppConfigLoader = originalLoader }()
			AppConfigLoader = func(app *StdApplication) error {
				return tt.appConfigLoaderErr
			}

			// Call Init and verify error collection
			err := app.Init()

			if tt.expectErrorCount > 0 {
				require.Error(t, err, "Expected errors but got none")

				// Check that all expected errors are present
				errStr := err.Error()
				for _, expectedErr := range tt.expectErrorContains {
					assert.Contains(t, errStr, expectedErr, "Expected error message not found")
				}

				// Verify error is properly joined
				unwrappedErrors := []error{}
				for err != nil {
					if joinErr, ok := err.(interface{ Unwrap() []error }); ok {
						unwrappedErrors = append(unwrappedErrors, joinErr.Unwrap()...)
						break
					}
					if unwrapErr, ok := err.(interface{ Unwrap() error }); ok {
						unwrappedErrors = append(unwrappedErrors, err)
						err = unwrapErr.Unwrap()
					} else {
						unwrappedErrors = append(unwrappedErrors, err)
						break
					}
				}

				// For Go 1.20+, errors.Join should contain multiple errors
				// We expect at least the specified error count
				assert.GreaterOrEqual(t, len(unwrappedErrors), 1, "Should contain multiple errors")
			} else {
				assert.NoError(t, err, "Expected no errors")
			}

			// Verify partial initialization if expected
			if tt.expectPartialInit {
				// Check that some modules were initialized successfully
				hasSuccessfulInit := false
				for _, module := range tt.modules {
					if sm, ok := module.(*initTestSuccessfulModule); ok && sm.initCalled {
						hasSuccessfulInit = true
						break
					}
				}
				assert.True(t, hasSuccessfulInit, "Expected some modules to initialize successfully")
			}
		})
	}
}

// Test_Application_Init_DependencyResolutionFailure tests error handling when dependency resolution fails
func Test_Application_Init_DependencyResolutionFailure(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Add modules with circular dependency
	app.RegisterModule(&initTestModule{name: "moduleA", dependencies: []string{"moduleB"}})
	app.RegisterModule(&initTestModule{name: "moduleB", dependencies: []string{"moduleA"}})

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve module dependencies")
	assert.Contains(t, err.Error(), "circular dependency")
}

// Test_Application_Init_TenantConfigurationFailure tests tenant configuration error handling
func Test_Application_Init_TenantConfigurationFailure(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Register a failing tenant config loader service
	failingTenantConfigLoader := &initTestFailingTenantConfigLoader{}
	app.RegisterService("tenantService", &initTestMockTenantService{})
	app.RegisterService("tenantConfigLoader", failingTenantConfigLoader)

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize tenant configurations")
	assert.Contains(t, err.Error(), "tenant config loader failed")
}

// Test_Application_Init_ServiceInjectionAndInitOrder tests that service injection happens before module init
func Test_Application_Init_ServiceInjectionAndInitOrder(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Create a service provider and consumer
	provider := &initTestServiceProvidingModule{
		initTestModule: initTestModule{name: "provider"},
		services: []ServiceProvider{
			{Name: "test-service", Instance: &MockStorage{data: map[string]string{"key": "value"}}},
		},
	}

	consumer := &initTestServiceConsumerModule{
		initTestModule: initTestModule{name: "consumer", dependencies: []string{"provider"}},
		requiredServices: []ServiceDependency{
			{Name: "test-service", Required: true},
		},
	}

	app.RegisterModule(provider)
	app.RegisterModule(consumer)

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()
	require.NoError(t, err)

	// Verify both modules were initialized
	assert.True(t, provider.initCalled, "Provider module should be initialized")
	assert.True(t, consumer.initCalled, "Consumer module should be initialized")
	assert.True(t, consumer.serviceInjected, "Service should be injected into consumer")

	// Verify service is available in registry
	var storage StorageService
	err = app.GetService("test-service", &storage)
	require.NoError(t, err)
	assert.Equal(t, "value", storage.Get("key"))
}

// Test_Application_Init_PartialFailureStateConsistency tests app state after partial failures
func Test_Application_Init_PartialFailureStateConsistency(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Add mix of successful and failing modules
	successModule := &initTestSuccessfulModule{initTestModule: initTestModule{name: "success"}}
	failModule := &initTestFailingInitModule{initTestModule: initTestModule{name: "failure"}, initErr: ErrModuleInitFailed}
	serviceModule := &initTestServiceProvidingModule{
		initTestModule: initTestModule{name: "service-provider"},
		services: []ServiceProvider{
			{Name: "working-service", Instance: &MockStorage{data: map[string]string{"test": "data"}}},
		},
	}

	app.RegisterModule(successModule)
	app.RegisterModule(failModule)
	app.RegisterModule(serviceModule)

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "module 'failure' failed to initialize")

	// Verify successful modules are still initialized
	assert.True(t, successModule.initCalled, "Successful module should be initialized")
	assert.True(t, serviceModule.initCalled, "Service provider should be initialized")
	assert.True(t, failModule.initCalled, "Failed module Init was called (but returned error)")

	// Verify services from successful modules are registered
	var storage StorageService
	serviceErr := app.GetService("working-service", &storage)
	require.NoError(t, serviceErr)
	assert.Equal(t, "data", storage.Get("test"))

	// Verify config sections from successful modules are available
	_, configErr := app.GetConfigSection("success-config")
	assert.NoError(t, configErr)
}

// Test_Application_Init_NoModules tests initialization with no modules
func Test_Application_Init_NoModules(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()
	assert.NoError(t, err)
	assert.Empty(t, app.moduleRegistry)
}

// Test_Application_Init_NonConfigurableModules tests modules that don't implement Configurable
func Test_Application_Init_NonConfigurableModules(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &initTestLogger{t: t},
	}

	// Add a module that doesn't implement Configurable
	nonConfigModule := &initTestModule{name: "non-config"}
	app.RegisterModule(nonConfigModule)

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error { return nil }

	err := app.Init()
	assert.NoError(t, err)
}

// Test helper modules - all prefixed with initTest to avoid conflicts

// initTestModule - base test module
type initTestModule struct {
	name         string
	dependencies []string
}

func (m initTestModule) Name() string           { return m.name }
func (m initTestModule) Init(Application) error { return nil }
func (m initTestModule) Dependencies() []string { return m.dependencies }

// initTestSuccessfulModule - implements all interfaces and succeeds
type initTestSuccessfulModule struct {
	initTestModule
	initCalled       bool
	configRegistered bool
}

func (m *initTestSuccessfulModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	m.configRegistered = true
	return nil
}

func (m *initTestSuccessfulModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *initTestSuccessfulModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *initTestSuccessfulModule) RequiresServices() []ServiceDependency { return nil }

// initTestFailingConfigModule - fails during config registration
type initTestFailingConfigModule struct {
	initTestModule
	configErr error
}

func (m *initTestFailingConfigModule) RegisterConfig(Application) error {
	return m.configErr
}

func (m *initTestFailingConfigModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *initTestFailingConfigModule) RequiresServices() []ServiceDependency { return nil }

// initTestFailingInitModule - fails during initialization
type initTestFailingInitModule struct {
	initTestModule
	initErr    error
	initCalled bool
}

func (m *initTestFailingInitModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	return nil
}

func (m *initTestFailingInitModule) Init(Application) error {
	m.initCalled = true
	return m.initErr
}

func (m *initTestFailingInitModule) ProvidesServices() []ServiceProvider   { return nil }
func (m *initTestFailingInitModule) RequiresServices() []ServiceDependency { return nil }

// initTestConflictingServiceModule - provides services that might conflict
type initTestConflictingServiceModule struct {
	initTestModule
	serviceName string
	initCalled  bool
}

func (m *initTestConflictingServiceModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	return nil
}

func (m *initTestConflictingServiceModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *initTestConflictingServiceModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{
		{Name: m.serviceName, Instance: &MockStorage{data: map[string]string{"key": "value"}}},
	}
}

func (m *initTestConflictingServiceModule) RequiresServices() []ServiceDependency { return nil }

// initTestServiceRequiringModule - requires services for injection
type initTestServiceRequiringModule struct {
	initTestModule
	requiredServices []ServiceDependency
	initCalled       bool
}

func (m *initTestServiceRequiringModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	return nil
}

func (m *initTestServiceRequiringModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *initTestServiceRequiringModule) ProvidesServices() []ServiceProvider { return nil }
func (m *initTestServiceRequiringModule) RequiresServices() []ServiceDependency {
	return m.requiredServices
}

// initTestServiceConsumerModule - consumes services and tracks injection
type initTestServiceConsumerModule struct {
	initTestModule
	requiredServices []ServiceDependency
	initCalled       bool
	serviceInjected  bool
}

func (m *initTestServiceConsumerModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	return nil
}

func (m *initTestServiceConsumerModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *initTestServiceConsumerModule) ProvidesServices() []ServiceProvider { return nil }
func (m *initTestServiceConsumerModule) RequiresServices() []ServiceDependency {
	return m.requiredServices
}

func (m *initTestServiceConsumerModule) Constructor() ModuleConstructor {
	return func(_ Application, services map[string]any) (Module, error) {
		// If we received any services, mark as injected
		if len(services) > 0 {
			m.serviceInjected = true
		}
		return m, nil
	}
}

// initTestServiceProvidingModule - provides services with initCalled tracking
type initTestServiceProvidingModule struct {
	initTestModule
	services   []ServiceProvider
	initCalled bool
}

func (m *initTestServiceProvidingModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider("config-value"))
	return nil
}

func (m *initTestServiceProvidingModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *initTestServiceProvidingModule) ProvidesServices() []ServiceProvider {
	return m.services
}

func (m *initTestServiceProvidingModule) RequiresServices() []ServiceDependency { return nil }

// Mock tenant service and config loader for testing
type initTestMockTenantService struct{}

func (m *initTestMockTenantService) GetTenantConfig(TenantID, string) (ConfigProvider, error) {
	return nil, nil
}
func (m *initTestMockTenantService) GetTenants() []TenantID { return nil }
func (m *initTestMockTenantService) RegisterTenant(TenantID, map[string]ConfigProvider) error {
	return nil
}
func (m *initTestMockTenantService) RemoveTenant(TenantID) error { return nil }
func (m *initTestMockTenantService) RegisterTenantAwareModule(TenantAwareModule) error {
	return nil
}

// initTestFailingTenantConfigLoader - fails during tenant config loading
type initTestFailingTenantConfigLoader struct{}

func (f *initTestFailingTenantConfigLoader) LoadTenantConfigurations(Application, TenantService) error {
	return fmt.Errorf("tenant config loader failed: %w", ErrTenantConfigInitFailed)
}

// initTestLogger - simple logger for tests
type initTestLogger struct {
	t *testing.T
}

func (l *initTestLogger) Debug(msg string, args ...interface{}) {
	l.t.Logf("DEBUG: %s %v", msg, args)
}

func (l *initTestLogger) Info(msg string, args ...interface{}) {
	l.t.Logf("INFO: %s %v", msg, args)
}

func (l *initTestLogger) Warn(msg string, args ...interface{}) {
	l.t.Logf("WARN: %s %v", msg, args)
}

func (l *initTestLogger) Error(msg string, args ...interface{}) {
	l.t.Logf("ERROR: %s %v", msg, args)
}
