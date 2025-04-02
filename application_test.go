package modular

import (
	"context"
	"reflect"
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

func (m *configRegisteringModule) RegisterConfig(app Application) {
	app.RegisterConfigSection(m.name+"-config", NewStdConfigProvider(m.name+"-config-value"))
	m.configRegistered = true
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
func (m testModule) RegisterConfig(Application)            {}
func (m testModule) ProvidesServices() []ServiceProvider   { return nil }
func (m testModule) RequiresServices() []ServiceDependency { return nil }
