package modular

import (
	"context"
	"reflect"
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

func Test_application_Init(t *testing.T) {
	// Setup standard config and logger for tests
	stdConfig := NewStdConfigProvider(testCfg{Str: "test"})
	stdLogger := &logger{t}

	// Create a test-only mock AppConfigLoader that does nothing
	originalLoader := AppConfigLoader
	defer func() { AppConfigLoader = originalLoader }()
	AppConfigLoader = func(app *StdApplication) error {
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

	tests := []struct {
		name          string
		cfgProvider   ConfigProvider
		cfgSections   map[string]ConfigProvider
		modules       []Module
		wantErr       bool
		errorContains string
		verify        func(t *testing.T, app *StdApplication)
	}{
		{
			name:        "basic initialization - no modules",
			cfgProvider: stdConfig,
			modules:     []Module{},
			wantErr:     false,
			verify: func(t *testing.T, app *StdApplication) {
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
			verify: func(t *testing.T, app *StdApplication) {
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
			verify: func(t *testing.T, app *StdApplication) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &StdApplication{
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
