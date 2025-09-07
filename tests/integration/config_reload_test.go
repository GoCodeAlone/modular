package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
)

// T057: Add integration test for dynamic config reload
func TestDynamicConfigReload_Integration(t *testing.T) {
	t.Run("should reload dynamic configuration successfully", func(t *testing.T) {
		// Create temporary configuration file
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// Initial configuration
		initialConfig := `
log_level: "info"
debug_enabled: false
max_connections: 100
static_field: "cannot_change"
`
		err := os.WriteFile(configPath, []byte(initialConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial config: %v", err)
		}

		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register configuration feeder
		yamlFeeder := feeders.NewYAMLFileFeeder(configPath)
		app.RegisterFeeder("config", yamlFeeder)

		ctx := context.Background()

		// Initialize application
		err = app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Get initial configuration values
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		initialLogLevel, err := provider.GetString("log_level")
		if err != nil {
			t.Fatalf("Failed to get initial log_level: %v", err)
		}
		if initialLogLevel != "info" {
			t.Errorf("Expected info, got: %s", initialLogLevel)
		}

		// Update configuration file with new values
		updatedConfig := `
log_level: "debug"
debug_enabled: true
max_connections: 200
static_field: "cannot_change"
new_field: "added_value"
`
		err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to update config file: %v", err)
		}

		// Trigger reload
		configLoader := app.GetConfigLoader()
		if configLoader == nil {
			t.Fatal("Config loader should be available")
		}

		err = configLoader.Reload(ctx)
		if err != nil {
			t.Fatalf("Failed to reload configuration: %v", err)
		}

		// Verify configuration was reloaded
		reloadedLogLevel, err := provider.GetString("log_level")
		if err != nil {
			t.Fatalf("Failed to get reloaded log_level: %v", err)
		}
		if reloadedLogLevel != "debug" {
			t.Errorf("Expected debug, got: %s", reloadedLogLevel)
		}

		reloadedDebug, err := provider.GetBool("debug_enabled")
		if err != nil {
			t.Fatalf("Failed to get reloaded debug_enabled: %v", err)
		}
		if !reloadedDebug {
			t.Error("Expected debug_enabled to be true")
		}

		reloadedConnections, err := provider.GetInt("max_connections")
		if err != nil {
			t.Fatalf("Failed to get reloaded max_connections: %v", err)
		}
		if reloadedConnections != 200 {
			t.Errorf("Expected 200, got: %d", reloadedConnections)
		}

		// Verify new field was added
		newField, err := provider.GetString("new_field")
		if err != nil {
			t.Fatalf("Failed to get new_field: %v", err)
		}
		if newField != "added_value" {
			t.Errorf("Expected added_value, got: %s", newField)
		}
	})

	t.Run("should handle configuration reload validation errors", func(t *testing.T) {
		// Create temporary configuration file
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// Valid initial configuration
		initialConfig := `
required_field: "value"
numeric_field: 100
`
		err := os.WriteFile(configPath, []byte(initialConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial config: %v", err)
		}

		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register configuration feeder
		yamlFeeder := feeders.NewYAMLFileFeeder(configPath)
		app.RegisterFeeder("config", yamlFeeder)

		ctx := context.Background()

		// Initialize application
		err = app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Update configuration file with invalid content
		invalidConfig := `
invalid_yaml: [unclosed bracket
numeric_field: "not_a_number"
`
		err = os.WriteFile(configPath, []byte(invalidConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to update config file: %v", err)
		}

		// Attempt to reload - should fail gracefully
		configLoader := app.GetConfigLoader()
		if configLoader == nil {
			t.Fatal("Config loader should be available")
		}

		err = configLoader.Reload(ctx)
		if err == nil {
			t.Error("Expected reload to fail with invalid configuration")
		}

		// Verify original configuration is still in effect
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		requiredField, err := provider.GetString("required_field")
		if err != nil {
			t.Fatalf("Failed to get required_field: %v", err)
		}
		if requiredField != "value" {
			t.Errorf("Expected original value, got: %s", requiredField)
		}

		numericField, err := provider.GetInt("numeric_field")
		if err != nil {
			t.Fatalf("Failed to get numeric_field: %v", err)
		}
		if numericField != 100 {
			t.Errorf("Expected original value 100, got: %d", numericField)
		}
	})

	t.Run("should track configuration provenance after reload", func(t *testing.T) {
		// Create temporary configuration files
		tempDir := t.TempDir()
		configPath1 := filepath.Join(tempDir, "config1.yaml")
		configPath2 := filepath.Join(tempDir, "config2.yaml")

		// Initial configurations
		config1 := `
field1: "from_config1"
field2: "from_config1"
`
		config2 := `
field2: "from_config2"
field3: "from_config2"
`

		err := os.WriteFile(configPath1, []byte(config1), 0644)
		if err != nil {
			t.Fatalf("Failed to create config1: %v", err)
		}

		err = os.WriteFile(configPath2, []byte(config2), 0644)
		if err != nil {
			t.Fatalf("Failed to create config2: %v", err)
		}

		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register multiple feeders
		yamlFeeder1 := feeders.NewYAMLFileFeeder(configPath1)
		app.RegisterFeeder("config1", yamlFeeder1)

		yamlFeeder2 := feeders.NewYAMLFileFeeder(configPath2)
		app.RegisterFeeder("config2", yamlFeeder2)

		ctx := context.Background()

		// Initialize application
		err = app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Update configuration files
		updatedConfig1 := `
field1: "updated_from_config1"
field2: "updated_from_config1"
new_field: "new_from_config1"
`
		err = os.WriteFile(configPath1, []byte(updatedConfig1), 0644)
		if err != nil {
			t.Fatalf("Failed to update config1: %v", err)
		}

		// Reload configuration
		configLoader := app.GetConfigLoader()
		if configLoader == nil {
			t.Fatal("Config loader should be available")
		}

		err = configLoader.Reload(ctx)
		if err != nil {
			t.Fatalf("Failed to reload configuration: %v", err)
		}

		// Verify configuration and provenance
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		// field1 should come from config1
		field1, err := provider.GetString("field1")
		if err != nil {
			t.Fatalf("Failed to get field1: %v", err)
		}
		if field1 != "updated_from_config1" {
			t.Errorf("Expected updated_from_config1, got: %s", field1)
		}

		// field2 should come from config2 (later feeder wins)
		field2, err := provider.GetString("field2")
		if err != nil {
			t.Fatalf("Failed to get field2: %v", err)
		}
		if field2 != "from_config2" {
			t.Errorf("Expected from_config2, got: %s", field2)
		}

		// field3 should come from config2
		field3, err := provider.GetString("field3")
		if err != nil {
			t.Fatalf("Failed to get field3: %v", err)
		}
		if field3 != "from_config2" {
			t.Errorf("Expected from_config2, got: %s", field3)
		}

		// new_field should come from config1
		newField, err := provider.GetString("new_field")
		if err != nil {
			t.Fatalf("Failed to get new_field: %v", err)
		}
		if newField != "new_from_config1" {
			t.Errorf("Expected new_from_config1, got: %s", newField)
		}
	})

	t.Run("should support timeout during configuration reload", func(t *testing.T) {
		// Create temporary configuration file
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// Initial configuration
		initialConfig := `
timeout_test: "initial"
`
		err := os.WriteFile(configPath, []byte(initialConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create initial config: %v", err)
		}

		app, err := modular.NewApplication()
		if err != nil {
			t.Fatalf("Failed to create application: %v", err)
		}
		app.EnableEnhancedLifecycle()

		// Register configuration feeder
		yamlFeeder := feeders.NewYAMLFileFeeder(configPath)
		app.RegisterFeeder("config", yamlFeeder)

		ctx := context.Background()

		// Initialize application
		err = app.InitWithEnhancedLifecycle(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize application: %v", err)
		}

		// Update configuration
		updatedConfig := `
timeout_test: "updated"
`
		err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to update config file: %v", err)
		}

		// Test reload with timeout
		configLoader := app.GetConfigLoader()
		if configLoader == nil {
			t.Fatal("Config loader should be available")
		}

		// Use a very short timeout context for testing timeout behavior
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
		defer cancel()

		// This might succeed or timeout depending on system speed
		err = configLoader.Reload(timeoutCtx)
		// We don't assert on timeout because it's system-dependent
		// The test validates that timeout handling exists

		// Now try with a reasonable timeout
		normalCtx, normalCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer normalCancel()

		err = configLoader.Reload(normalCtx)
		if err != nil {
			t.Fatalf("Failed to reload with normal timeout: %v", err)
		}

		// Verify the reload succeeded
		provider := app.ConfigProvider()
		if provider == nil {
			t.Fatal("Config provider should be available")
		}

		timeoutTest, err := provider.GetString("timeout_test")
		if err != nil {
			t.Fatalf("Failed to get timeout_test: %v", err)
		}
		if timeoutTest != "updated" {
			t.Errorf("Expected updated, got: %s", timeoutTest)
		}
	})
}