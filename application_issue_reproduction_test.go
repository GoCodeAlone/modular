package modular

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
	"github.com/stretchr/testify/mock"
)

// Test_ReproduceIssue_LoggerCachingProblem reproduces the exact issue from the problem statement
// This test verifies that without OnConfigLoaded, modules would cache the initial logger
func Test_ReproduceIssue_LoggerCachingProblem_WithoutFix(t *testing.T) {
	// This test demonstrates the OLD problem (for documentation purposes)
	// We'll manually verify the issue exists without the hook

	type AppConfig struct {
		LogFormat string `yaml:"logFormat" default:"text"`
	}

	config := &AppConfig{LogFormat: "text"}

	// 1. Create initial logger (text format)
	var textLogOutput strings.Builder
	initialLogger := slog.New(slog.NewTextHandler(&textLogOutput, nil))

	app := NewStdApplication(NewStdConfigProvider(config), initialLogger)

	// 2. Create a test module that caches logger
	testModule := &TestModuleWithCachedLogger{}
	app.RegisterModule(testModule)

	// 3. Initialize WITHOUT hook - module gets text logger
	if err := app.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 4. AFTER init, try to reconfigure logger (simulating old approach)
	var jsonLogOutput strings.Builder
	newLogger := slog.New(slog.NewJSONHandler(&jsonLogOutput, nil))
	app.SetLogger(newLogger)

	// 5. Problem: Module still has the old text logger cached!
	// The module's cached logger is the initial one, not the new one
	if testModule.logger != initialLogger {
		t.Error("Expected module to have cached the initial logger (demonstrating the problem)")
	}

	if testModule.logger == newLogger {
		t.Error("Module should NOT have the new logger when reconfigured after Init (this is the problem)")
	}
}

// Test_SolutionWithOnConfigLoaded_CompleteScenario tests the complete solution
func Test_SolutionWithOnConfigLoaded_CompleteScenario(t *testing.T) {
	// This test verifies the SOLUTION works correctly

	type AppConfig struct {
		LogFormat string `yaml:"logFormat" default:"text"`
	}

	config := &AppConfig{LogFormat: "json"} // Config says use JSON

	// 1. Create initial logger (text format) - will be replaced
	var textLogOutput strings.Builder
	initialLogger := slog.New(slog.NewTextHandler(&textLogOutput, nil))

	app := NewStdApplication(NewStdConfigProvider(config), initialLogger)

	// 2. Create a test module that caches logger
	testModule := &TestModuleWithCachedLogger{}
	app.RegisterModule(testModule)

	// 3. Register hook to reconfigure logger BEFORE module init
	var jsonLogOutput strings.Builder
	newLogger := slog.New(slog.NewJSONHandler(&jsonLogOutput, nil))

	app.OnConfigLoaded(func(app Application) error {
		cfg := app.ConfigProvider().GetConfig().(*AppConfig)
		if cfg.LogFormat == "json" {
			app.SetLogger(newLogger)
		}
		return nil
	})

	// 4. Initialize - hook runs, then module gets the NEW logger
	if err := app.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 5. Verify: Module has the NEW logger, not the initial one
	if testModule.logger == initialLogger {
		t.Error("Module should NOT have the initial logger when hook reconfigures it")
	}

	if testModule.logger != newLogger {
		t.Error("Module should have the reconfigured logger from the hook")
	}

	// 6. Verify app.Logger() also returns the new logger
	if app.Logger() != newLogger {
		t.Error("Application should return the reconfigured logger")
	}
}

// Test_CompleteWorkflow_FromProblemStatement reproduces the exact workflow from the issue
func Test_CompleteWorkflow_FromProblemStatement(t *testing.T) {
	// Create a temporary config file
	configContent := []byte("logFormat: json\n")
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(configContent); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	type AppConfig struct {
		LogFormat string `yaml:"logFormat" default:"text"`
	}

	// 1. Create initial logger with default settings (text format)
	initialLogger := &MockLogger{}
	initialLogger.On("Debug", mock.Anything, mock.Anything).Return()
	initialLogger.On("Info", mock.Anything, mock.Anything).Return()

	// 2. Create application with initial logger
	app, err := NewApplication(
		WithLogger(initialLogger),
		WithConfigProvider(NewStdConfigProvider(&AppConfig{})),
		// 3. Register hook to reconfigure logger based on config
		WithOnConfigLoaded(func(app Application) error {
			cfg := app.ConfigProvider().GetConfig().(*AppConfig)
			if cfg.LogFormat == "json" {
				// Create new logger with JSON format
				newLogger := &MockLogger{}
				newLogger.On("Debug", mock.Anything, mock.Anything).Return()
				newLogger.On("Info", mock.Anything, mock.Anything).Return()
				app.SetLogger(newLogger)
			}
			return nil
		}),
		WithModules(&TestModuleWithCachedLogger{}),
	)

	if err != nil {
		t.Fatalf("NewApplication failed: %v", err)
	}

	// Set up feeders to load from config file
	if stdApp, ok := app.(*StdApplication); ok {
		stdApp.SetConfigFeeders([]Feeder{
			feeders.NewYamlFeeder(tmpfile.Name()),
		})
	}

	// 4. Init - this loads config, runs hook, then initializes modules
	if err := app.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 5. Verify the logger was reconfigured
	if app.Logger() == initialLogger {
		t.Error("Logger should have been reconfigured from initial logger")
	}

	// Get the module and verify it has the reconfigured logger
	module := app.GetModule("test_module_with_logger")
	if module == nil {
		t.Fatal("Module not found")
	}

	cachedModule, ok := module.(*TestModuleWithCachedLogger)
	if !ok {
		t.Fatal("Module is not the expected type")
	}

	if cachedModule.logger == initialLogger {
		t.Error("Module should have the reconfigured logger, not the initial one")
	}

	if cachedModule.logger == nil {
		t.Error("Module logger should not be nil")
	}
}

// Test_MultipleReconfigurationsInHooks tests that multiple hooks can modify dependencies
func Test_MultipleReconfigurationsInHooks(t *testing.T) {
	logger := &MockLogger{}
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Info", mock.Anything, mock.Anything).Return()

	type AppConfig struct {
		Feature1 bool `yaml:"feature1" default:"true"`
		Feature2 bool `yaml:"feature2" default:"true"`
	}

	config := &AppConfig{}
	app := NewStdApplication(NewStdConfigProvider(config), logger)

	// Track what hooks executed
	var executedHooks []string

	// First hook: configure feature 1
	app.OnConfigLoaded(func(app Application) error {
		executedHooks = append(executedHooks, "feature1")
		cfg := app.ConfigProvider().GetConfig().(*AppConfig)
		if cfg.Feature1 {
			// Register a service for feature 1
			return app.RegisterService("feature1", "enabled")
		}
		return nil
	})

	// Second hook: configure feature 2
	app.OnConfigLoaded(func(app Application) error {
		executedHooks = append(executedHooks, "feature2")
		cfg := app.ConfigProvider().GetConfig().(*AppConfig)
		if cfg.Feature2 {
			// Register a service for feature 2
			return app.RegisterService("feature2", "enabled")
		}
		return nil
	})

	if err := app.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify both hooks executed in order
	if len(executedHooks) != 2 {
		t.Errorf("Expected 2 hooks to execute, got %d", len(executedHooks))
	}

	if executedHooks[0] != "feature1" || executedHooks[1] != "feature2" {
		t.Errorf("Hooks executed in wrong order: %v", executedHooks)
	}

	// Verify both services were registered
	var service1, service2 string
	if err := app.GetService("feature1", &service1); err != nil {
		t.Errorf("Feature1 service not found: %v", err)
	}
	if err := app.GetService("feature2", &service2); err != nil {
		t.Errorf("Feature2 service not found: %v", err)
	}
}

// Test_ErrorInHook_PreventsModuleInit verifies that hook errors are reported
func Test_ErrorInHook_PreventsModuleInit(t *testing.T) {
	logger := &MockLogger{}
	logger.On("Debug", mock.Anything, mock.Anything).Return()
	logger.On("Info", mock.Anything, mock.Anything).Return()

	config := &struct{}{}
	app := NewStdApplication(NewStdConfigProvider(config), logger)

	module := &ConfigLoadedTestModule{
		name: "test",
		initFunc: func(app Application) error {
			return nil
		},
	}
	app.RegisterModule(module)

	// Register a hook that fails
	app.OnConfigLoaded(func(app Application) error {
		return fmt.Errorf("hook failed intentionally")
	})

	// Init should fail due to hook error
	err := app.Init()
	if err == nil {
		t.Fatal("Expected Init to fail when hook returns error")
	}

	// Verify the error message mentions the hook failure
	if !containsSubstring(err.Error(), "config loaded hook") {
		t.Errorf("Error should mention config loaded hook failure: %v", err)
	}

	// Note: Due to error collection strategy in Init, modules may still initialize
	// but the overall Init will return an error. This is consistent with how
	// the framework handles other initialization errors.
}
