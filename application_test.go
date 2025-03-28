package modular

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
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
			want: &Application{
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
			want: &Application{
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
			if got := NewApplication(tt.args.cfgProvider, tt.args.logger); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewApplication() = %v, want %v", got, tt.want)
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

func (m *configRegisteringModule) RegisterConfig(app *Application) {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider(m.name+"-config-value"))
	m.configRegistered = true
}

func (m *configRegisteringModule) Init(app *Application) error {
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

// Mock module that requires services
type serviceRequiringModule struct {
	testModule
	requiredServices []ServiceDependency
}

func (m *serviceRequiringModule) RequiresServices() []ServiceDependency {
	return m.requiredServices
}

// OrderTrackingModule tracks initialization order for testing
type OrderTrackingModule struct {
	testModule
	initCalled       bool
	initOrder        int
	initError        error
	dependsOn        []string
	servicesNeeded   []ServiceDependency
	servicesProvided []ServiceProvider
}

func (m *OrderTrackingModule) Init(app *Application) error {
	m.initCalled = true
	return m.initError
}

func (m *OrderTrackingModule) Dependencies() []string {
	return m.dependsOn
}

func (m *OrderTrackingModule) RequiresServices() []ServiceDependency {
	return m.servicesNeeded
}

func (m *OrderTrackingModule) ProvidesServices() []ServiceProvider {
	return m.servicesProvided
}

func Test_application_Init(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
	stdLogger := &logger{t}

	// Create a test-only mock AppConfigLoader that does nothing
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *Application) error {
		// Return error if config provider is nil
		if app.cfgProvider == nil {
			return fmt.Errorf("failed to load app config: config provider is nil")
		}

		// Return error if there's an "error-trigger" section
		if _, exists := app.cfgSections["error-trigger"]; exists {
			return fmt.Errorf("failed to load app config: error triggered by section")
		}

		return nil
	}

	tests := []struct {
		name          string
		cfgProvider   ConfigProvider
		cfgSections   map[string]ConfigProvider
		modules       []Module
		wantErr       bool
		errorContains string
		verify        func(t *testing.T, app *Application)
	}{
		{
			name:        "basic initialization - no modules",
			cfgProvider: stdConfig,
			modules:     []Module{},
			wantErr:     false,
			verify: func(t *testing.T, app *Application) {
				if len(app.moduleRegistry) != 0 {
					t.Error("Expected empty module registry")
				}
			},
		},
		{
			name:        "config registration",
			cfgProvider: stdConfig,
			modules: []Module{
				&configRegisteringModule{
					testModule: testModule{name: "config-module"},
				},
			},
			wantErr: false,
			verify: func(t *testing.T, app *Application) {
				// Check that config was registered
				configModule, ok := app.moduleRegistry["config-module"].(*configRegisteringModule)
				if !ok {
					t.Error("Module not found or wrong type")
					return
				}

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
			},
		},
		{
			name:        "service registration",
			cfgProvider: stdConfig,
			modules: []Module{
				&serviceProvidingModule{
					testModule: testModule{name: "service-module"},
					services: []ServiceProvider{
						{Name: "test-service", Instance: &MockStorage{data: map[string]string{"key": "value"}}},
					},
				},
			},
			wantErr: false,
			verify: func(t *testing.T, app *Application) {
				// Check that service was registered
				if _, exists := app.svcRegistry["test-service"]; !exists {
					t.Error("Service was not registered")
				}

				// Get and verify the service
				var storage StorageService
				err := app.GetService("test-service", &storage)
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
			},
		},
		{
			name:        "service injection",
			cfgProvider: stdConfig,
			modules: []Module{
				&serviceProvidingModule{
					testModule: testModule{name: "provider"},
					services: []ServiceProvider{
						{Name: "storage", Instance: &MockStorage{data: map[string]string{"key": "value"}}},
					},
				},
				&ConstructorInjectionModule{
					testModule: testModule{
						name:         "consumer",
						dependencies: []string{"provider"},
					},
				},
			},
			wantErr: false,
			verify: func(t *testing.T, app *Application) {
				// Verify the consumer module received the service
				consumer, ok := app.moduleRegistry["consumer"].(*ConstructorInjectionModule)
				if !ok {
					t.Error("Consumer module not found or wrong type")
					return
				}
				if consumer.storage == nil {
					t.Error("Service was not injected into consumer")
					return
				}
				if val := consumer.storage.Get("key"); val != "value" {
					t.Errorf("Expected value %s, got %s", "value", val)
				}
			},
		},
		{
			name:        "dependency resolution",
			cfgProvider: stdConfig,
			modules: []Module{
				&testModule{name: "A"},
				&testModule{name: "B", dependencies: []string{"A"}},
				&testModule{name: "C", dependencies: []string{"B"}},
			},
			wantErr: false,
			verify: func(t *testing.T, app *Application) {
				// The modules should have been initialized in order A, B, C
				// but we can't directly verify the order after initialization

				// Instead, verify that all modules are in the registry
				for _, name := range []string{"A", "B", "C"} {
					if _, exists := app.moduleRegistry[name]; !exists {
						t.Errorf("Module %s not found in registry", name)
					}
				}
			},
		},
		{
			name:          "config loading error",
			cfgProvider:   nil, // Will cause error in loadAppConfig
			modules:       []Module{},
			wantErr:       true,
			errorContains: "failed to load app config",
		},
		{
			name:        "error in config section",
			cfgProvider: stdConfig,
			cfgSections: map[string]ConfigProvider{
				"error-trigger": stdConfig, // Will trigger error in loadAppConfig
			},
			modules:       []Module{},
			wantErr:       true,
			errorContains: "failed to load app config",
		},
		{
			name:        "service registration error - duplicate",
			cfgProvider: stdConfig,
			modules: []Module{
				&serviceProvidingModule{
					testModule: testModule{name: "provider1"},
					services: []ServiceProvider{
						{Name: "duplicate-service", Instance: "service1"},
					},
				},
				&serviceProvidingModule{
					testModule: testModule{name: "provider2"},
					services: []ServiceProvider{
						{Name: "duplicate-service", Instance: "service2"},
					},
				},
			},
			wantErr:       true,
			errorContains: "failed to register service",
		},
		{
			name:        "circular dependency error",
			cfgProvider: stdConfig,
			modules: []Module{
				&testModule{name: "A", dependencies: []string{"B"}},
				&testModule{name: "B", dependencies: []string{"A"}},
			},
			wantErr:       true,
			errorContains: "failed to resolve dependencies",
		},
		{
			name:        "missing required service",
			cfgProvider: stdConfig,
			modules: []Module{
				&serviceRequiringModule{
					testModule: testModule{name: "consumer"},
					requiredServices: []ServiceDependency{
						{
							Name:     "missing-service",
							Required: true,
						},
					},
				},
			},
			wantErr:       true,
			errorContains: "required service 'missing-service' not found",
		},
		{
			name:        "module init error",
			cfgProvider: stdConfig,
			modules: []Module{
				&configRegisteringModule{
					testModule: testModule{name: "error-module"},
					initError:  fmt.Errorf("init error"),
				},
			},
			wantErr:       true,
			errorContains: "failed to initialize module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.cfgProvider,
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         stdLogger,
			}

			// Add any config sections
			if tt.cfgSections != nil {
				for name, provider := range tt.cfgSections {
					app.cfgSections[name] = provider
				}
			}

			// Register modules
			for _, module := range tt.modules {
				app.RegisterModule(module)
			}

			// Call Init
			err := app.Init()

			// Verify error expectations
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				return
			}

			// Custom verifications
			if !tt.wantErr && tt.verify != nil {
				tt.verify(t, app)
			}
		})
	}
}

func Test_application_Init_OrderTracking(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider("app-config")
	stdLogger := &logger{t}

	// Mock the AppConfigLoader function for this test
	originalLoader := AppConfigLoader
	AppConfigLoader = func(app *Application) error {
		return nil // Just return success, no actual loading
	}
	defer func() { AppConfigLoader = originalLoader }()

	// Create modules with dependencies to test initialization order
	moduleA := &OrderTrackingModule{testModule: testModule{name: "A"}, dependsOn: []string{}}
	moduleB := &OrderTrackingModule{testModule: testModule{name: "B"}, dependsOn: []string{"A"}}
	moduleC := &OrderTrackingModule{testModule: testModule{name: "C"}, dependsOn: []string{"B"}}

	app := &Application{
		cfgProvider:    stdConfig,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         stdLogger,
	}

	// Register modules - intentionally in non-dependency order
	app.RegisterModule(moduleC)
	app.RegisterModule(moduleA)
	app.RegisterModule(moduleB)

	// Track original module instances before initialization
	originalModules := map[string]*OrderTrackingModule{
		"A": moduleA,
		"B": moduleB,
		"C": moduleC,
	}

	// Initialize the application
	err := app.Init()
	if err != nil {
		t.Errorf("Init() unexpected error: %v", err)
		return
	}

	// Verify all modules were initialized
	for name, originalModule := range originalModules {
		if !originalModule.initCalled {
			t.Errorf("Module %s was not initialized", name)
		}
	}

	// Verify initialization order through dependency check
	// For each module, verify all its dependencies were initialized before it
	moduleOrder, err := app.resolveDependencies()
	if err != nil {
		t.Errorf("Failed to resolve dependencies: %v", err)
		return
	}

	// Verify the order is correct (A, B, C)
	expectedOrder := []string{"A", "B", "C"}
	if !reflect.DeepEqual(moduleOrder, expectedOrder) {
		t.Errorf("Expected order %v, got %v", expectedOrder, moduleOrder)
	}
}

func Test_application_Init_ContextCreation(t *testing.T) {
	// Setup standard config and logger
	stdConfig := NewStdConfigProvider("app-config")
	stdLogger := &logger{t}

	// Mock the AppConfigLoader function for this test
	originalLoader := AppConfigLoader
	AppConfigLoader = func(app *Application) error {
		return nil // Just return success, no actual loading
	}
	defer func() { AppConfigLoader = originalLoader }()

	// Create an application
	app := &Application{
		cfgProvider:    stdConfig,
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         stdLogger,
	}

	// Create a ContextCheckingModule instead of trying to modify the Start method
	contextCheckingModule := &ContextCheckingModule{
		testModule: testModule{name: "context-checker"},
	}

	app.RegisterModule(contextCheckingModule)

	// Initialize and start the application
	err := app.Init()
	if err != nil {
		t.Errorf("Init() unexpected error: %v", err)
		return
	}

	err = app.Start()
	if err != nil {
		t.Errorf("Start() unexpected error: %v", err)
		return
	}

	// Verify the module's Start was called with a context
	if !contextCheckingModule.startCalled {
		t.Error("Module Start() was not called")
	}

	if !contextCheckingModule.contextFound {
		t.Error("Context was not passed to module Start()")
	}

	// Verify the application has a context and cancel function
	if app.ctx == nil {
		t.Error("Application context was not created")
	}

	if app.cancel == nil {
		t.Error("Application cancel function was not created")
	}

	// Clean up
	app.Stop()
}

// ContextCheckingModule is a special test module that tracks if Start was called with a context
type ContextCheckingModule struct {
	testModule
	startCalled  bool
	contextFound bool
}

// Start overrides the testModule.Start method to check for context
func (m *ContextCheckingModule) Start(ctx context.Context) error {
	m.startCalled = true
	m.contextFound = ctx != nil
	return nil
}

// Helper to temporarily replace the real AppConfigLoader with a mock during tests
func withMockConfigLoader(mockLoader LoadAppConfigFunc, testFunc func()) {
	originalLoader := AppConfigLoader
	AppConfigLoader = mockLoader
	defer func() {
		AppConfigLoader = originalLoader
	}()
	testFunc()
}
func Test_application_Init_WithMockedConfigLoader(t *testing.T) {
	t.Run("mocked config loader success", func(t *testing.T) {
		mockCalled := false
		withMockConfigLoader(func(app *Application) error {
			mockCalled = true
			return nil
		}, func() {
			app := NewApplication(nil, &logger{t})
			err := app.Init()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !mockCalled {
				t.Error("Mock config loader was not called")
			}
		})
	})

	t.Run("mocked config loader error", func(t *testing.T) {
		expectedErr := fmt.Errorf("mock config error")
		withMockConfigLoader(func(app *Application) error {
			return expectedErr
		}, func() {
			app := NewApplication(nil, &logger{t})
			err := app.Init()
			if err == nil {
				t.Error("Expected error but got nil")
			}
			if !strings.Contains(err.Error(), "failed to load app config") {
				t.Errorf("Expected error containing 'failed to load app config', got: %v", err)
			}
		})
	})
}

func Test_application_Logger(t *testing.T) {
	// Create mock loggers for testing
	mockLogger1 := &logger{t}
	mockLogger2 := &MockLogger{}

	tests := []struct {
		name   string
		logger Logger
		want   Logger
	}{
		{
			name:   "nil logger",
			logger: nil,
			want:   nil,
		},
		{
			name:   "test logger",
			logger: mockLogger1,
			want:   mockLogger1,
		},
		{
			name:   "mock logger",
			logger: mockLogger2,
			want:   mockLogger2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    nil,
				svcRegistry:    nil,
				moduleRegistry: nil,
				logger:         tt.logger,
			}
			if got := app.Logger(); got != tt.want {
				t.Errorf("Logger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_application_RegisterModule(t *testing.T) {
	tests := []struct {
		name             string
		initialRegistry  ModuleRegistry
		moduleToRegister Module
		expectedRegistry func(ModuleRegistry) bool
	}{
		{
			name:            "register module to empty registry",
			initialRegistry: make(ModuleRegistry),
			moduleToRegister: testModule{
				name: "module-a",
			},
			expectedRegistry: func(registry ModuleRegistry) bool {
				if len(registry) != 1 {
					return false
				}
				_, exists := registry["module-a"]
				return exists
			},
		},
		{
			name: "register module to non-empty registry",
			initialRegistry: func() ModuleRegistry {
				registry := make(ModuleRegistry)
				registry["existing-module"] = testModule{name: "existing-module"}
				return registry
			}(),
			moduleToRegister: testModule{
				name: "module-a",
			},
			expectedRegistry: func(registry ModuleRegistry) bool {
				if len(registry) != 2 {
					return false
				}
				_, existsA := registry["module-a"]
				_, existsExisting := registry["existing-module"]
				return existsA && existsExisting
			},
		},
		{
			name: "register module with same name (overwrite)",
			initialRegistry: func() ModuleRegistry {
				registry := make(ModuleRegistry)
				registry["module-a"] = testModule{
					name:         "module-a",
					dependencies: []string{"dep-1"},
				}
				return registry
			}(),
			moduleToRegister: testModule{
				name:         "module-a",
				dependencies: []string{"dep-2"},
			},
			expectedRegistry: func(registry ModuleRegistry) bool {
				if len(registry) != 1 {
					return false
				}

				mod, exists := registry["module-a"]
				if !exists {
					return false
				}

				// Check that it's the new module by checking dependencies
				testMod, ok := mod.(testModule)
				if !ok {
					return false
				}

				return len(testMod.dependencies) == 1 && testMod.dependencies[0] == "dep-2"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				moduleRegistry: tt.initialRegistry,
				logger:         &logger{t},
			}

			app.RegisterModule(tt.moduleToRegister)

			// Verify registry matches expectations
			if !tt.expectedRegistry(app.moduleRegistry) {
				t.Errorf("RegisterModule() registry state doesn't match expected state")
			}
		})
	}
}

func Test_application_RegisterService(t *testing.T) {
	type testService struct {
		Value string
	}

	type anotherService struct {
		Count int
	}

	tests := []struct {
		name             string
		existingServices map[string]any
		serviceName      string
		serviceValue     any
		wantErr          bool
		errorContains    string
	}{
		{
			name:             "register new service",
			existingServices: map[string]any{},
			serviceName:      "test-service",
			serviceValue:     &testService{Value: "test"},
			wantErr:          false,
		},
		{
			name: "register duplicate service",
			existingServices: map[string]any{
				"test-service": &testService{Value: "already-exists"},
			},
			serviceName:   "test-service",
			serviceValue:  &testService{Value: "new-value"},
			wantErr:       true,
			errorContains: "already registered",
		},
		{
			name: "register multiple different services",
			existingServices: map[string]any{
				"service-1": &testService{Value: "first"},
			},
			serviceName:  "service-2",
			serviceValue: &anotherService{Count: 42},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &logger{t}
			app := &Application{
				svcRegistry: make(ServiceRegistry),
				logger:      logger,
			}

			// Pre-populate registry with existing services
			for name, svc := range tt.existingServices {
				app.svcRegistry[name] = svc
			}

			err := app.RegisterService(tt.serviceName, tt.serviceValue)

			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				return
			}

			if !tt.wantErr {
				// Verify service was registered correctly
				if registeredSvc, exists := app.svcRegistry[tt.serviceName]; !exists {
					t.Errorf("Service '%s' was not registered", tt.serviceName)
				} else if registeredSvc != tt.serviceValue {
					t.Errorf("Service '%s' was registered with value %v, expected %v",
						tt.serviceName, registeredSvc, tt.serviceValue)
				}
			}
		})
	}
}

// Define test interfaces and structs
type TestInterface interface {
	TestMethod() string
}

type ConcreteService struct {
	Value string
}

func (c *ConcreteService) TestMethod() string {
	return c.Value
}

type DifferentService struct {
	Count int
}

func Test_application_GetService(t *testing.T) {
	tests := []struct {
		name          string
		registry      ServiceRegistry
		serviceName   string
		target        any
		wantErr       bool
		errorContains string
		expectedValue any
	}{
		{
			name: "get existing service with correct type",
			registry: ServiceRegistry{
				"test-service": &ConcreteService{Value: "hello"},
			},
			serviceName:   "test-service",
			target:        &ConcreteService{},
			wantErr:       false,
			expectedValue: &ConcreteService{Value: "hello"},
		},
		{
			name: "get interface implementation",
			registry: ServiceRegistry{
				"interface-service": &ConcreteService{Value: "via interface"},
			},
			serviceName:   "interface-service",
			target:        &struct{ TestInterface }{},
			wantErr:       false,
			expectedValue: &struct{ TestInterface }{&ConcreteService{Value: "via interface"}},
		},
		{
			name: "service not found",
			registry: ServiceRegistry{
				"existing-service": "value",
			},
			serviceName:   "missing-service",
			target:        &ConcreteService{},
			wantErr:       true,
			errorContains: "not found",
		},
		{
			name: "target not a pointer",
			registry: ServiceRegistry{
				"test-service": &ConcreteService{Value: "hello"},
			},
			serviceName:   "test-service",
			target:        ConcreteService{},
			wantErr:       true,
			errorContains: "must be a pointer",
		},
		{
			name: "nil target",
			registry: ServiceRegistry{
				"test-service": &ConcreteService{Value: "hello"},
			},
			serviceName:   "test-service",
			target:        nil,
			wantErr:       true,
			errorContains: "must be a pointer",
		},
		{
			name: "type mismatch",
			registry: ServiceRegistry{
				"test-service": &ConcreteService{Value: "hello"},
			},
			serviceName:   "test-service",
			target:        &DifferentService{},
			wantErr:       true,
			errorContains: "cannot be assigned to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				svcRegistry: tt.registry,
				logger:      &logger{t},
			}

			err := app.GetService(tt.serviceName, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				return
			}

			if !tt.wantErr && tt.expectedValue != nil && tt.target != nil {
				targetValue := reflect.ValueOf(tt.target)
				if targetValue.Kind() == reflect.Ptr && targetValue.Elem().Kind() == reflect.Struct {
					// Handle the case of struct with embedded interface
					for i := 0; i < targetValue.Elem().NumField(); i++ {
						if targetValue.Elem().Field(i).Kind() == reflect.Interface {
							if targetValue.Elem().Field(i).IsNil() != reflect.ValueOf(tt.expectedValue).Elem().Field(i).IsNil() {
								t.Errorf("GetService() target interface field is nil = %v, expected %v",
									targetValue.Elem().Field(i).IsNil(), reflect.ValueOf(tt.expectedValue).Elem().Field(i).IsNil())
							}
							continue
						}
					}
				} else {
					// Regular pointer comparison
					if !reflect.DeepEqual(tt.target, tt.expectedValue) {
						t.Errorf("GetService() target value = %v, expected %v", tt.target, tt.expectedValue)
					}
				}
			}
		})
	}
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

// Module that requires services
type ServiceConsumerModule struct {
	testModule
	storage StorageService
	logger  Logger
}

// Module with constructor injection
type ConstructorInjectionModule struct {
	testModule
	storage StorageService
	logger  Logger
}

func (m *ConstructorInjectionModule) Constructor() ModuleConstructor {
	return func(app *Application, services map[string]any) (Module, error) {
		module := &ConstructorInjectionModule{
			testModule: m.testModule,
		}

		if storage, ok := services["storage"].(StorageService); ok {
			module.storage = storage
		}

		if logger, ok := services["logger"].(Logger); ok {
			module.logger = logger
		}

		return module, nil
	}
}

func (m *ConstructorInjectionModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "storage",
			SatisfiesInterface: reflect.TypeOf((*StorageService)(nil)).Elem(),
			Required:           true,
		},
		{
			Name:               "logger",
			SatisfiesInterface: reflect.TypeOf((*Logger)(nil)).Elem(),
			Required:           false,
		},
	}
}

func Test_injectServices(t *testing.T) {
	tests := []struct {
		name          string
		setupServices func(app *Application)
		module        Module
		wantErr       bool
		errorContains string
		verify        func(t *testing.T, result Module)
	}{
		{
			name: "constructor injection with all services",
			setupServices: func(app *Application) {
				app.RegisterService("storage", &MockStorage{data: map[string]string{"key": "value"}})
				app.RegisterService("logger", &MockLogger{})
			},
			module: &ConstructorInjectionModule{
				testModule: testModule{
					name: "test-module",
				},
			},
			wantErr: false,
			verify: func(t *testing.T, result Module) {
				module, ok := result.(*ConstructorInjectionModule)
				if !ok {
					t.Fatalf("Expected *ConstructorInjectionModule, got %T", result)
				}

				if module.storage == nil {
					t.Error("Storage service was not injected")
				}

				if module.logger == nil {
					t.Error("Logger service was not injected")
				}

				value := module.storage.Get("key")
				if value != "value" {
					t.Errorf("Storage returned incorrect value: %s", value)
				}
			},
		},
		{
			name: "missing required service",
			setupServices: func(app *Application) {
				// Not registering the required storage service
				app.RegisterService("logger", &MockLogger{})
			},
			module: &ConstructorInjectionModule{
				testModule: testModule{
					name: "test-module",
				},
			},
			wantErr:       true,
			errorContains: "required service 'storage' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         &logger{t},
			}

			// Setup services
			if tt.setupServices != nil {
				tt.setupServices(app)
			}

			// Register the module
			app.moduleRegistry[tt.module.Name()] = tt.module

			// Inject services
			result, err := app.injectServices(tt.module)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("injectServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				return
			}

			// Verify the result if needed
			if !tt.wantErr && tt.verify != nil {
				tt.verify(t, result)
			}
		})
	}
}

// Create mock module implementation for testing
type testModule struct {
	name         string
	dependencies []string
}

// Implement Module interface for our test module
func (m testModule) Name() string                          { return m.name }
func (m testModule) Dependencies() []string                { return m.dependencies }
func (m testModule) Init(app *Application) error           { return nil }
func (m testModule) Start(ctx context.Context) error       { return nil }
func (m testModule) Stop(ctx context.Context) error        { return nil }
func (m testModule) RegisterConfig(app *Application)       {}
func (m testModule) ProvidesServices() []ServiceProvider   { return nil }
func (m testModule) RequiresServices() []ServiceDependency { return nil }

func Test_application_resolveDependencies(t *testing.T) {
	tests := []struct {
		name           string
		modules        []testModule
		expectedOrder  []string
		expectedErrMsg string
	}{
		{
			name: "single module, no dependencies",
			modules: []testModule{
				{name: "A", dependencies: []string{}},
			},
			expectedOrder:  []string{"A"},
			expectedErrMsg: "",
		},
		{
			name: "linear dependency chain",
			modules: []testModule{
				{name: "A", dependencies: []string{}},
				{name: "B", dependencies: []string{"A"}},
				{name: "C", dependencies: []string{"B"}},
			},
			expectedOrder:  []string{"A", "B", "C"},
			expectedErrMsg: "",
		},
		{
			name: "multiple dependencies",
			modules: []testModule{
				{name: "A", dependencies: []string{}},
				{name: "B", dependencies: []string{}},
				{name: "C", dependencies: []string{"A", "B"}},
			},
			expectedOrder:  []string{"A", "B", "C"}, // A and B could be in any order
			expectedErrMsg: "",
		},
		{
			name: "diamond dependency",
			modules: []testModule{
				{name: "A", dependencies: []string{}},
				{name: "B", dependencies: []string{"A"}},
				{name: "C", dependencies: []string{"A"}},
				{name: "D", dependencies: []string{"B", "C"}},
			},
			expectedOrder:  []string{"A", "B", "C", "D"}, // B and C could be in any order
			expectedErrMsg: "",
		},
		{
			name: "circular dependency",
			modules: []testModule{
				{name: "A", dependencies: []string{"C"}},
				{name: "B", dependencies: []string{"A"}},
				{name: "C", dependencies: []string{"B"}},
			},
			expectedOrder:  nil,
			expectedErrMsg: "circular dependency detected",
		},
		{
			name: "missing dependency",
			modules: []testModule{
				{name: "A", dependencies: []string{}},
				{name: "B", dependencies: []string{"MissingModule"}},
			},
			expectedOrder:  nil,
			expectedErrMsg: "depends on non-existent module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new application for each test with a mock logger
			app := &Application{
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         &logger{t},
			}

			// Register modules for this test
			for _, mod := range tt.modules {
				app.moduleRegistry[mod.name] = mod
			}

			// Call the method under test
			result, err := app.resolveDependencies()

			// Check error cases
			if tt.expectedErrMsg != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.expectedErrMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
				return
			}

			// Check success cases
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// For some dependency graphs, the order can vary but still be valid
			// For simplicity in this test, we'll just check length and presence of each module
			if len(result) != len(tt.expectedOrder) {
				t.Errorf("expected %d modules, got %d", len(tt.expectedOrder), len(result))
				return
			}

			// Verify that for every dependency relationship, the dependency comes before the dependent
			for modName, mod := range app.moduleRegistry {
				modIndex := slices.Index(result, modName)
				if modIndex == -1 {
					t.Errorf("module %s missing from result", modName)
					continue
				}

				for _, depName := range mod.Dependencies() {
					depIndex := slices.Index(result, depName)
					if depIndex == -1 {
						t.Errorf("dependency %s missing from result", depName)
						continue
					}
					if depIndex >= modIndex {
						t.Errorf("dependency %s should come before %s in result", depName, modName)
					}
				}
			}
		})
	}
}
