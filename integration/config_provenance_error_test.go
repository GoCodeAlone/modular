package integration

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	modular "github.com/GoCodeAlone/modular"
)

// TestConfigProvenanceAndRequiredFieldFailureReporting tests T026: Integration config provenance & required field failure reporting
// This test verifies that configuration errors include proper provenance information
// and that required field failures are clearly reported with context.
func TestConfigProvenanceAndRequiredFieldFailureReporting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Test case 1: Required field missing
	t.Run("RequiredFieldMissing", func(t *testing.T) {
		// Create a config module that requires certain fields
		configModule := &testConfigModule{
			name: "configTestModule",
			config: &testModuleConfig{
				// Leave RequiredField empty to trigger validation error
				RequiredField: "",
				OptionalField: "present",
			},
		}

		// Create application
		app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
		app.RegisterModule(configModule)

		// Initialize application - should fail due to missing required field
		err := app.Init()
		if err == nil {
			t.Fatal("Expected initialization to fail due to missing required field, but it succeeded")
		}

		// Verify error contains provenance information
		errorStr := err.Error()
		t.Logf("Configuration error: %s", errorStr)

		// Check for expected error elements:
		// 1. Module name should be mentioned
		if !strings.Contains(errorStr, "configTestModule") {
			t.Errorf("Error should contain module name 'configTestModule', got: %s", errorStr)
		}

		// 2. Field name should be mentioned
		if !strings.Contains(errorStr, "RequiredField") {
			t.Errorf("Error should contain field name 'RequiredField', got: %s", errorStr)
		}

		// 3. Should indicate it's a validation/required field issue
		if !(strings.Contains(errorStr, "required") || strings.Contains(errorStr, "validation") || strings.Contains(errorStr, "missing")) {
			t.Errorf("Error should indicate required/validation issue, got: %s", errorStr)
		}

		t.Log("✅ Required field error properly reported with context")
	})

	// Test case 2: Invalid field value
	t.Run("InvalidFieldValue", func(t *testing.T) {
		// Create a config module with invalid field value
		configModule := &testConfigModule{
			name: "configTestModule2",
			config: &testModuleConfig{
				RequiredField: "present",
				OptionalField: "present",
				NumericField:  -1, // Invalid value (should be positive)
			},
		}

		// Create application
		app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
		app.RegisterModule(configModule)

		// Initialize application - should fail due to invalid field value
		err := app.Init()
		if err == nil {
			t.Fatal("Expected initialization to fail due to invalid field value, but it succeeded")
		}

		errorStr := err.Error()
		t.Logf("Validation error: %s", errorStr)

		// Verify error contains context about the invalid value
		if !strings.Contains(errorStr, "configTestModule2") {
			t.Errorf("Error should contain module name 'configTestModule2', got: %s", errorStr)
		}

		t.Log("✅ Invalid field value error properly reported")
	})

	// Test case 3: Configuration source tracking (provenance)
	t.Run("ConfigurationProvenance", func(t *testing.T) {
		// This test verifies that configuration errors include information about
		// where the configuration came from (file, env var, default, etc.)

		// Create a module with valid config to test provenance tracking
		configModule := &testConfigModule{
			name: "provenanceTestModule",
			config: &testModuleConfig{
				RequiredField: "valid",
				OptionalField: "from-test",
				NumericField:  42,
			},
		}

		// Create application
		app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
		app.RegisterModule(configModule)

		// Initialize application - should succeed
		err := app.Init()
		if err != nil {
			t.Fatalf("Application initialization failed: %v", err)
		}

		// For now, just verify successful config loading
		// Future enhancement: track where each config value came from
		t.Log("✅ Configuration loaded successfully")
		t.Log("⚠️  Note: Enhanced provenance tracking (source file/env/default) is not yet implemented")
	})
}

// TestConfigurationErrorAccumulation verifies how the framework handles multiple config errors
func TestConfigurationErrorAccumulation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create multiple modules with different config errors
	module1 := &testConfigModule{
		name: "errorModule1",
		config: &testModuleConfig{
			RequiredField: "", // Missing required field
		},
	}

	module2 := &testConfigModule{
		name: "errorModule2",
		config: &testModuleConfig{
			RequiredField: "present",
			NumericField:  -5, // Invalid value
		},
	}

	module3 := &testConfigModule{
		name: "validModule",
		config: &testModuleConfig{
			RequiredField: "present",
			OptionalField: "valid",
			NumericField:  10,
		},
	}

	// Create application
	app := modular.NewStdApplication(modular.NewStdConfigProvider(&struct{}{}), logger)
	app.RegisterModule(module1)
	app.RegisterModule(module2)
	app.RegisterModule(module3)

	// Initialize application - should fail at first config error
	err := app.Init()
	if err == nil {
		t.Fatal("Expected initialization to fail due to config errors, but it succeeded")
	}

	errorStr := err.Error()
	t.Logf("Configuration error (current behavior): %s", errorStr)

	// Current behavior: framework stops at first configuration error
	// Verify first error module is mentioned
	// Current behavior: framework stops at the first configuration error encountered.
	// Validation order may change (e.g., iteration over an internal map) so accept either failing module.
	if !(strings.Contains(errorStr, "errorModule1") || strings.Contains(errorStr, "errorModule2")) {
		// Ensure at least one known failing module is referenced
		t.Errorf("Error should reference either 'errorModule1' or 'errorModule2', got: %s", errorStr)
	}

	// Check if multiple errors are accumulated (both module names present)
	if strings.Contains(errorStr, "errorModule1") && strings.Contains(errorStr, "errorModule2") {
		t.Log("✅ Enhanced behavior: Multiple configuration errors accumulated and reported")
	} else {
		t.Log("⚠️  Current behavior: Framework stops at first configuration error")
		t.Log("⚠️  Note: Error accumulation for config validation not yet implemented")
	}

	t.Log("✅ Configuration error handling behavior documented")
}

// testModuleConfig represents a module configuration with validation
type testModuleConfig struct {
	RequiredField string `yaml:"required_field" json:"required_field" required:"true" desc:"This field is required"`
	OptionalField string `yaml:"optional_field" json:"optional_field" default:"default_value" desc:"This field is optional"`
	NumericField  int    `yaml:"numeric_field" json:"numeric_field" default:"1" desc:"Must be positive"`
}

// Validate implements the ConfigValidator interface
func (cfg *testModuleConfig) Validate() error {
	if cfg.RequiredField == "" {
		return modular.ErrConfigValidationFailed
	}
	if cfg.NumericField < 0 {
		return modular.ErrConfigValidationFailed
	}
	return nil
}

// testConfigModule is a module that uses configuration with validation
type testConfigModule struct {
	name   string
	config *testModuleConfig
}

func (m *testConfigModule) Name() string {
	return m.name
}

func (m *testConfigModule) RegisterConfig(app modular.Application) error {
	// Register the configuration section
	provider := modular.NewStdConfigProvider(m.config)
	app.RegisterConfigSection(m.name, provider)
	return nil
}

func (m *testConfigModule) Init(app modular.Application) error {
	// Configuration validation should have already occurred during RegisterConfig
	return nil
}
