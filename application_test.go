package modular

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"testing"
)

func TestNewApplication(t *testing.T) {
	type args struct {
		cfgProvider ConfigProvider
		logger      Logger
	}
	cp := NewStdConfigProvider(testCfg{Str: "test"})
	log := &logger{}
	tests := []struct {
		name string
		args args
		want AppRegistry
	}{
		{
			name: "TestNewApplication",
			args: args{
				cfgProvider: nil,
				logger:      nil,
			},
			want: &StdApplication{
				cfgProvider:    nil,
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         nil,
			},
		},
		{
			name: "TestNewApplicationWithConfigProviderAndLogger",
			args: args{
				cfgProvider: cp,
				logger:      log,
			},
			want: &StdApplication{
				cfgProvider:    cp,
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         log,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStdApplication(tt.args.cfgProvider, tt.args.logger); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStdApplication() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Mock module for testing configuration registration
type configRegisteringModule struct {
	testModule
	configRegistered bool
	initCalled       bool
	initError        error
}

func (m *configRegisteringModule) RegisterConfig(app Application) error {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider(m.name+"-config-value"))
	m.configRegistered = true
	return nil
}

func (m *configRegisteringModule) Init(Application) error {
	m.initCalled = true
	return m.initError
}

// Mock module that provides services
type serviceProvidingModule struct {
	testModule
	services []ServiceProvider
}

func (m *serviceProvidingModule) ProvidesServices() []ServiceProvider {
	return m.services
}

// Test_application_Init_NoModules tests initialization with no modules
func Test_application_Init_NoModules(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
	stdLogger := &logger{t}

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = testAppConfigLoader

	app := &StdApplication{
		cfgProvider:    stdConfig,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         stdLogger,
	}

	// Call Init
	err := app.Init()
	if err != nil {
		t.Errorf("Init() error = %v, expected no error", err)
		return
	}

	// Verify
	if len(app.moduleRegistry) != 0 {
		t.Error("Expected empty module registry")
	}
}

// Test_application_Init_ConfigRegistration tests module config registration
func Test_application_Init_ConfigRegistration(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
	stdLogger := &logger{t}

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = testAppConfigLoader

	configModule := &configRegisteringModule{
		testModule: testModule{name: "config-module"},
	}

	app := &StdApplication{
		cfgProvider:    stdConfig,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         stdLogger,
	}

	// Register modules
	app.RegisterModule(configModule)

	// Call Init
	err := app.Init()
	if err != nil {
		t.Errorf("Init() error = %v, expected no error", err)
		return
	}

	// Verify config was registered
	if !configModule.configRegistered {
		t.Error("RegisterConfig was not called on module")
	}

	// Verify config section was added
	section, err := app.GetConfigSection("config-module-config")
	if err != nil {
		t.Errorf("Config section not found: %v", err)
	}
	if section == nil {
		t.Error("Config section is nil")
	}

	// Verify init was called
	if !configModule.initCalled {
		t.Error("Init was not called on module")
	}
}

// Test_application_Init_ServiceRegistration tests service registration
func Test_application_Init_ServiceRegistration(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
	stdLogger := &logger{t}

	// Setup mock AppConfigLoader
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = testAppConfigLoader

	app := &StdApplication{
		cfgProvider:    stdConfig,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         stdLogger,
	}

	serviceModule := &serviceProvidingModule{
		testModule: testModule{name: "service-module"},
		services: []ServiceProvider{
			{Name: "test-service", Instance: &MockStorage{data: map[string]string{"key": "value"}}},
		},
	}

	// Register modules
	app.RegisterModule(serviceModule)

	// Call Init
	err := app.Init()
	if err != nil {
		t.Errorf("Init() error = %v, expected no error", err)
		return
	}

	// Check that service was registered
	if _, exists := app.svcRegistry["test-service"]; !exists {
		t.Error("Service was not registered")
	}

	// Get and verify the service
	var storage StorageService
	err = app.GetService("test-service", &storage)
	if err != nil {
		t.Errorf("Failed to get service: %v", err)
	}
	if storage == nil {
		t.Error("Retrieved service is nil")
		return
	}
	if val := storage.Get("key"); val != "value" {
		t.Errorf("Expected value %s, got %s", "value", val)
	}
}

// Helper function for testing AppConfigLoader
func testAppConfigLoader(app *StdApplication) error {
	// Return error if config provider is nil
	if app.cfgProvider == nil {
		return ErrConfigProviderNil
	}

	// Return error if there's an "error-trigger" section
	if _, exists := app.cfgSections["error-trigger"]; exists {
		return ErrConfigSectionError
	}

	return nil
}

// Define test service interfaces and implementations
type StorageService interface {
	Get(key string) string
}

type MockStorage struct {
	data map[string]string
}

func (m *MockStorage) Get(key string) string {
	return m.data[key]
}

// Create mock module implementation for testing
type testModule struct {
	name         string
	dependencies []string
}

// Implement Module interface for our test module
func (m testModule) Name() string                          { return m.name }
func (m testModule) Dependencies() []string                { return m.dependencies }
func (m testModule) Init(Application) error                { return nil }
func (m testModule) Start(context.Context) error           { return nil }
func (m testModule) Stop(context.Context) error            { return nil }
func (m testModule) RegisterConfig(Application) error      { return nil }
func (m testModule) ProvidesServices() []ServiceProvider   { return nil }
func (m testModule) RequiresServices() []ServiceDependency { return nil }

// Test_RegisterService tests service registration scenarios
func Test_RegisterService(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &logger{t},
	}

	// Test successful registration
	err := app.RegisterService("storage", &MockStorage{data: map[string]string{"key": "value"}})
	if err != nil {
		t.Errorf("RegisterService() error = %v, expected no error", err)
	}

	// Test duplicate registration
	err = app.RegisterService("storage", &MockStorage{data: map[string]string{}})
	if err == nil {
		t.Error("RegisterService() expected error for duplicate service, got nil")
	} else if !IsServiceAlreadyRegisteredError(err) {
		t.Errorf("RegisterService() expected ErrServiceAlreadyRegistered, got %v", err)
	}
}

// Test_GetService tests service retrieval scenarios
func Test_GetService(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &logger{t},
	}

	// Register test services
	mockStorage := &MockStorage{data: map[string]string{"key": "value"}}
	if err := app.RegisterService("storage", mockStorage); err != nil {
		t.Fatalf("Failed to register storage service: %v", err)
	}

	// Test retrieving existing service
	tests := []struct {
		name        string
		serviceName string
		target      interface{}
		wantErr     bool
		errCheck    func(error) bool
	}{
		{
			name:        "Get existing service with interface target",
			serviceName: "storage",
			target:      new(StorageService),
			wantErr:     false,
		},
		{
			name:        "Get existing service with concrete type target",
			serviceName: "storage",
			target:      new(MockStorage),
			wantErr:     false,
		},
		{
			name:        "Get non-existent service",
			serviceName: "unknown",
			target:      new(StorageService),
			wantErr:     true,
			errCheck:    IsServiceNotFoundError,
		},
		{
			name:        "Target not a pointer",
			serviceName: "storage",
			target:      StorageService(nil),
			wantErr:     true,
			errCheck:    func(err error) bool { return errors.Is(err, ErrTargetNotPointer) },
		},
		{
			name:        "Incompatible target type",
			serviceName: "storage",
			target:      new(string),
			wantErr:     true,
			errCheck:    IsServiceIncompatibleError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.GetService(tt.serviceName, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("GetService() expected specific error, got %v", err)
			}

			if !tt.wantErr {
				if ptr, ok := tt.target.(*StorageService); ok && *ptr == nil {
					t.Error("GetService() service was nil after successful retrieval")
				}
			}
		})
	}
}

// Test_ResolveDependencies tests module dependency resolution
func Test_ResolveDependencies(t *testing.T) {
	tests := []struct {
		name       string
		modules    []Module
		wantErr    bool
		errCheck   func(error) bool
		checkOrder func([]string) bool
	}{
		{
			name: "Simple dependency chain",
			modules: []Module{
				&testModule{name: "module-c", dependencies: []string{"module-b"}},
				&testModule{name: "module-b", dependencies: []string{"module-a"}},
				&testModule{name: "module-a", dependencies: []string{}},
			},
			wantErr: false,
			checkOrder: func(order []string) bool {
				// Ensure module-a comes before module-b and module-b before module-c
				aIdx := -1
				bIdx := -1
				cIdx := -1
				for i, name := range order {
					switch name {
					case "module-a":
						aIdx = i
					case "module-b":
						bIdx = i
					case "module-c":
						cIdx = i
					}
				}
				return aIdx < bIdx && bIdx < cIdx
			},
		},
		{
			name: "Circular dependency",
			modules: []Module{
				&testModule{name: "module-a", dependencies: []string{"module-b"}},
				&testModule{name: "module-b", dependencies: []string{"module-a"}},
			},
			wantErr:  true,
			errCheck: IsCircularDependencyError,
		},
		{
			name: "Missing dependency",
			modules: []Module{
				&testModule{name: "module-a", dependencies: []string{"non-existent"}},
			},
			wantErr:  true,
			errCheck: IsModuleDependencyMissingError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &StdApplication{
				cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         &logger{t},
			}

			// Register modules
			for _, module := range tt.modules {
				app.RegisterModule(module)
			}

			// Resolve dependencies
			order, err := app.resolveDependencies()

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("resolveDependencies() expected specific error, got %v", err)
			}

			if !tt.wantErr && tt.checkOrder != nil && !tt.checkOrder(order) {
				t.Errorf("resolveDependencies() returned incorrect order: %v", order)
			}
		})
	}
}

// Mock module that tracks lifecycle methods
type lifecycleTestModule struct {
	testModule
	initCalled  bool
	startCalled bool
	stopCalled  bool
	startError  error
	stopError   error
}

func (m *lifecycleTestModule) Init(Application) error {
	m.initCalled = true
	return nil
}

func (m *lifecycleTestModule) Start(context.Context) error {
	m.startCalled = true
	return m.startError
}

func (m *lifecycleTestModule) Stop(context.Context) error {
	m.stopCalled = true
	return m.stopError
}

// Test_ApplicationLifecycle tests the Start and Stop methods
func Test_ApplicationLifecycle(t *testing.T) {
	// Test successful Start and Stop
	t.Run("Successful lifecycle", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		module1 := &lifecycleTestModule{testModule: testModule{name: "module1"}}
		module2 := &lifecycleTestModule{testModule: testModule{name: "module2", dependencies: []string{"module1"}}}

		app.RegisterModule(module1)
		app.RegisterModule(module2)

		// Test Start
		if err := app.Start(); err != nil {
			t.Errorf("Start() error = %v, expected no error", err)
		}

		// Verify context was created
		if app.ctx == nil {
			t.Error("Start() did not create application context")
		}

		// Verify modules were started in correct order
		if !module1.startCalled {
			t.Error("Start() did not call Start on first module")
		}
		if !module2.startCalled {
			t.Error("Start() did not call Start on second module")
		}

		// Test Stop
		if err := app.Stop(); err != nil {
			t.Errorf("Stop() error = %v, expected no error", err)
		}

		// Verify modules were stopped (should be in reverse order)
		if !module1.stopCalled {
			t.Error("Stop() did not call Stop on first module")
		}
		if !module2.stopCalled {
			t.Error("Stop() did not call Stop on second module")
		}
	})

	// Test Start failure
	t.Run("Start failure", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
		}

		failingModule := &lifecycleTestModule{
			testModule: testModule{name: "failing"},
			startError: ErrModuleStartFailed,
		}

		app.RegisterModule(failingModule)

		// Test Start
		if err := app.Start(); err == nil {
			t.Error("Start() expected error for failing module, got nil")
		}
	})

	// Test Stop with error
	t.Run("Stop with error", func(t *testing.T) {
		app := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         slog.Default(),
		}

		failingModule := &lifecycleTestModule{
			testModule: testModule{name: "failing"},
			stopError:  ErrModuleStopFailed,
		}

		app.RegisterModule(failingModule)

		// Start first so we can test Stop
		if err := app.Start(); err != nil {
			t.Fatalf("Start() error = %v, expected no error", err)
		}

		// Test Stop - should return error but continue stopping
		if err := app.Stop(); err == nil {
			t.Error("Stop() expected error for failing module, got nil")
		}
	})
}

// Test_TenantFunctionality tests tenant-related methods
func Test_TenantFunctionality(t *testing.T) {
	// Setup tenant service and configs
	tenantSvc := &mockTenantService{
		tenantConfigs: map[TenantID]map[string]ConfigProvider{
			"tenant1": {
				"app": NewStdConfigProvider(testCfg{Str: "tenant1-config"}),
			},
		},
	}

	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &logger{t},
		ctx:            context.Background(),
	}

	// Register tenant service
	if err := app.RegisterService("tenantService", tenantSvc); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}
	if err := app.RegisterService("tenantConfigLoader", NewFileBasedTenantConfigLoader(TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`.*\.json$`),
		ConfigDir:       "",
		ConfigFeeders:   nil,
	})); err != nil {
		t.Fatalf("Failed to register tenant config loader: %v", err)
	}

	// Test GetTenantService
	t.Run("GetTenantService", func(t *testing.T) {
		ts, err := app.GetTenantService()
		if err != nil {
			t.Errorf("GetTenantService() error = %v, expected no error", err)
			return
		}
		if ts == nil {
			t.Error("GetTenantService() returned nil service")
		}
	})

	// Test WithTenant
	t.Run("WithTenant", func(t *testing.T) {
		tctx, err := app.WithTenant("tenant1")
		if err != nil {
			t.Errorf("WithTenant() error = %v, expected no error", err)
			return
		}
		if tctx == nil {
			t.Error("WithTenant() returned nil context")
			return
		}
		if tctx.GetTenantID() != "tenant1" {
			t.Errorf("WithTenant() tenantID = %v, expected tenant1", tctx.GetTenantID())
		}
	})

	// Test WithTenant with nil context
	t.Run("WithTenant with nil context", func(t *testing.T) {
		appWithNoCtx := &StdApplication{
			cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
			cfgSections:    make(map[string]ConfigProvider),
			svcRegistry:    make(ServiceRegistry),
			moduleRegistry: make(ModuleRegistry),
			logger:         &logger{t},
			ctx:            nil, // No context initialized
		}

		_, err := appWithNoCtx.WithTenant("tenant1")
		if err == nil {
			t.Error("WithTenant() expected error for nil app context, got nil")
		}
	})

	// Test GetTenantConfig
	t.Run("GetTenantConfig", func(t *testing.T) {
		cfg, err := app.GetTenantConfig("tenant1", "app")
		if err != nil {
			t.Errorf("GetTenantConfig() error = %v, expected no error", err)
			return
		}
		if cfg == nil {
			t.Error("GetTenantConfig() returned nil config")
			return
		}

		// Verify the config content
		tcfg, ok := cfg.GetConfig().(testCfg)
		if !ok {
			t.Errorf("Failed to get structured config: %v", err)
			return
		}
		if tcfg.Str != "tenant1-config" {
			t.Errorf("Expected config value 'tenant1-config', got '%s'", tcfg.Str)
		}
	})

	// Test GetTenantConfig for non-existent tenant
	t.Run("GetTenantConfig for non-existent tenant", func(t *testing.T) {
		_, err := app.GetTenantConfig("non-existent", "app")
		if err == nil {
			t.Error("GetTenantConfig() expected error for non-existent tenant, got nil")
		}
	})
}

// Helper for error checking
func IsServiceAlreadyRegisteredError(err error) bool {
	return err != nil && ErrorIs(err, ErrServiceAlreadyRegistered)
}

func IsServiceNotFoundError(err error) bool {
	return err != nil && ErrorIs(err, ErrServiceNotFound)
}

func IsServiceIncompatibleError(err error) bool {
	return err != nil && ErrorIs(err, ErrServiceIncompatible)
}

func IsCircularDependencyError(err error) bool {
	return err != nil && ErrorIs(err, ErrCircularDependency)
}

func IsModuleDependencyMissingError(err error) bool {
	return err != nil && ErrorIs(err, ErrModuleDependencyMissing)
}

// ErrorIs is a helper function that checks if err contains target error
func ErrorIs(err, target error) bool {
	// Simple implementation that checks if target is in err's chain
	for {
		if errors.Is(err, target) {
			return true
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
			if err == nil {
				return false
			}
		} else {
			return false
		}
	}
}

// Test_ApplicationSetLogger tests the SetLogger functionality
func Test_ApplicationSetLogger(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Verify initial logger is set
	if app.Logger() != initialLogger {
		t.Error("Initial logger not set correctly")
	}

	// Create a new logger
	newLogger := &logger{t}

	// Set the new logger
	app.SetLogger(newLogger)

	// Verify the logger was changed
	if app.Logger() != newLogger {
		t.Error("SetLogger did not update the logger correctly")
	}

	// Verify the old logger is no longer referenced
	if app.Logger() == initialLogger {
		t.Error("SetLogger did not replace the old logger")
	}
}

// Test_ApplicationSetLoggerWithNilLogger tests SetLogger with nil logger
func Test_ApplicationSetLoggerWithNilLogger(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Set logger to nil
	app.SetLogger(nil)

	// Verify logger is now nil
	if app.Logger() != nil {
		t.Error("SetLogger did not set logger to nil correctly")
	}
}

// Test_ApplicationSetLoggerRuntimeUsage tests runtime logger switching with actual usage
func Test_ApplicationSetLoggerRuntimeUsage(t *testing.T) {
	// Create initial logger
	initialLogger := &logger{t}

	// Create application with initial logger
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		initialLogger,
	)

	// Verify initial logger is set
	if app.Logger() != initialLogger {
		t.Error("Initial logger not set correctly")
	}

	// Create a new mock logger to switch to
	newMockLogger := &MockLogger{}
	// Set up a simple expectation that might be called later
	newMockLogger.On("Debug", "Test message", []interface{}{"key", "value"}).Return().Maybe()

	// Switch to the new logger
	app.SetLogger(newMockLogger)

	// Verify the logger was switched
	if app.Logger() != newMockLogger {
		t.Error("Logger was not switched correctly")
	}

	// Verify the old logger is no longer referenced
	if app.Logger() == initialLogger {
		t.Error("SetLogger did not replace the old logger")
	}

	// Test that the new logger is actually used when the application logs something
	app.Logger().Debug("Test message", "key", "value")

	// Verify mock expectations were met (if any were called)
	newMockLogger.AssertExpectations(t)
}

// Placeholder errors for tests
var (
	ErrModuleStartFailed = fmt.Errorf("module start failed")
	ErrModuleStopFailed  = fmt.Errorf("module stop failed")
)

func TestSetVerboseConfig(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{
			name:    "Enable verbose config",
			enabled: true,
		},
		{
			name:    "Disable verbose config",
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock logger to capture debug messages
			mockLogger := &MockLogger{}

			// Set up expectations for debug messages
			if tt.enabled {
				mockLogger.On("Debug", "Verbose configuration debugging enabled", []interface{}(nil)).Return()
			} else {
				mockLogger.On("Debug", "Verbose configuration debugging disabled", []interface{}(nil)).Return()
			}

			// Create application with mock logger
			app := NewStdApplication(
				NewStdConfigProvider(testCfg{Str: "test"}),
				mockLogger,
			)

			// Test that verbose config is initially false
			if app.IsVerboseConfig() != false {
				t.Error("Expected verbose config to be initially false")
			}

			// Set verbose config
			app.SetVerboseConfig(tt.enabled)

			// Verify the setting was applied
			if app.IsVerboseConfig() != tt.enabled {
				t.Errorf("Expected verbose config to be %v, got %v", tt.enabled, app.IsVerboseConfig())
			}

			// Verify mock expectations were met
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestIsVerboseConfig(t *testing.T) {
	mockLogger := &MockLogger{}

	// Create application
	app := NewStdApplication(
		NewStdConfigProvider(testCfg{Str: "test"}),
		mockLogger,
	)

	// Test initial state
	if app.IsVerboseConfig() != false {
		t.Error("Expected IsVerboseConfig to return false initially")
	}

	// Test after enabling
	mockLogger.On("Debug", "Verbose configuration debugging enabled", []interface{}(nil)).Return()
	app.SetVerboseConfig(true)
	if app.IsVerboseConfig() != true {
		t.Error("Expected IsVerboseConfig to return true after enabling")
	}

	// Test after disabling
	mockLogger.On("Debug", "Verbose configuration debugging disabled", []interface{}(nil)).Return()
	app.SetVerboseConfig(false)
	if app.IsVerboseConfig() != false {
		t.Error("Expected IsVerboseConfig to return false after disabling")
	}

	mockLogger.AssertExpectations(t)
}
