package modular

import (
	"os"
	"strings"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setEnvWithCleanup sets an environment variable and schedules it for cleanup using t.Cleanup
func setEnvWithCleanup(t *testing.T, key, value string) {
	t.Setenv(key, value)
	// Also ensure it gets unset when the test finishes
	t.Cleanup(func() {
		os.Unsetenv(key)
	})
}

// setEnvVarsWithCleanup sets multiple environment variables with automatic cleanup
func setEnvVarsWithCleanup(t *testing.T, envVars map[string]string) {
	var keys []string
	for key, value := range envVars {
		t.Setenv(key, value)
		keys = append(keys, key)
	}

	// Schedule cleanup of all variables when test finishes
	t.Cleanup(func() {
		for _, key := range keys {
			os.Unsetenv(key)
		}
	})
}

// createTestConfig creates a new config with field tracking for tests
func createTestConfig() (*Config, FieldTracker, *MockLogger) {
	// Create logger that captures debug output
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create field tracker
	tracker := NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Create configuration builder with field tracking
	cfg := NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	cfg.SetFieldTracker(tracker)

	return cfg, tracker, mockLogger
}

// clearTestEnvironment clears all environment variables that could affect our tests
func clearTestEnvironment(t *testing.T) {
	// Clear all potential test environment variables
	testEnvVars := []string{
		// Test 1 variables
		"TEST1_APP_NAME", "TEST1_APP_DEBUG", "TEST1_APP_PORT",
		// Test 2 variables
		"TEST2_DB_PRIMARY_DRIVER", "TEST2_DB_PRIMARY_DSN", "TEST2_DB_PRIMARY_MAX_CONNS",
		"TEST2_DB_SECONDARY_DRIVER", "TEST2_DB_SECONDARY_DSN", "TEST2_DB_SECONDARY_MAX_CONNS",
		// Test 3 variables
		"TEST3_APP_NAME", "TEST3_APP_DEBUG", "TEST3_APP_PORT",
		"TEST3_DB_PRIMARY_DSN", "TEST3_DB_PRIMARY_DRIVER", "TEST3_DB_PRIMARY_MAX_CONNS",
		"TEST3_DB_SECONDARY_DSN", "TEST3_DB_SECONDARY_DRIVER", "TEST3_DB_SECONDARY_MAX_CONNS",
		// Legacy env vars (just in case)
		"APP_NAME", "APP_DEBUG", "APP_PORT",
		"DB_PRIMARY_DRIVER", "DB_PRIMARY_DSN", "DB_PRIMARY_MAX_CONNS",
		"DB_SECONDARY_DRIVER", "DB_SECONDARY_DSN", "DB_SECONDARY_MAX_CONNS",
	}

	for _, key := range testEnvVars {
		os.Unsetenv(key)
	}
}

// TestBasicEnvFeederWithConfigBuilder tests field tracking with basic environment feeder
func TestBasicEnvFeederWithConfigBuilder(t *testing.T) {
	// Clear any environment variables from previous tests
	clearTestEnvironment(t)

	// Set up environment variables for this specific test
	setEnvVarsWithCleanup(t, map[string]string{
		"TEST1_APP_NAME":  "Test App",
		"TEST1_APP_DEBUG": "true",
		"TEST1_APP_PORT":  "8080",
	})

	cfg, tracker, mockLogger := createTestConfig()

	// Define a basic app config
	type AppConfig struct {
		AppName string `env:"TEST1_APP_NAME"`
		Debug   bool   `env:"TEST1_APP_DEBUG"`
		Port    int    `env:"TEST1_APP_PORT"`
	}

	config := &AppConfig{}

	// Add environment feeder
	envFeeder := feeders.NewEnvFeeder()
	cfg.AddFeeder(envFeeder)

	// Add the configuration structure
	cfg.AddStructKey("app", config)

	// Feed configuration
	err := cfg.Feed()
	require.NoError(t, err)

	// Debug: Check if field tracker has any populations
	populations := tracker.(*DefaultFieldTracker).FieldPopulations
	t.Logf("Field tracker has %d populations after feeding", len(populations))
	for i, pop := range populations {
		t.Logf("  %d: %s -> %v (from %s:%s)", i, pop.FieldPath, pop.Value, pop.SourceType, pop.SourceKey)
	}

	// Verify config was populated correctly
	assert.Equal(t, "Test App", config.AppName)
	assert.Equal(t, true, config.Debug)
	assert.Equal(t, 8080, config.Port)

	// Verify field tracking captured all field populations
	require.GreaterOrEqual(t, len(populations), 3, "Should track at least 3 field populations")

	// Verify specific field populations
	appNamePop := findFieldPopulation(populations, "AppName")
	require.NotNil(t, appNamePop, "AppName field population should be tracked")
	assert.Equal(t, "Test App", appNamePop.Value)
	assert.Equal(t, "env", appNamePop.SourceType)
	assert.Equal(t, "TEST1_APP_NAME", appNamePop.SourceKey)
	assert.Contains(t, appNamePop.SearchKeys, "TEST1_APP_NAME")

	debugPop := findFieldPopulation(populations, "Debug")
	require.NotNil(t, debugPop, "Debug field population should be tracked")
	assert.Equal(t, true, debugPop.Value)
	assert.Equal(t, "env", debugPop.SourceType)
	assert.Equal(t, "TEST1_APP_DEBUG", debugPop.SourceKey)

	portPop := findFieldPopulation(populations, "Port")
	require.NotNil(t, portPop, "Port field population should be tracked")
	assert.Equal(t, 8080, portPop.Value)
	assert.Equal(t, "env", portPop.SourceType)
	assert.Equal(t, "TEST1_APP_PORT", portPop.SourceKey)

	// Verify that field tracking was used and logged
	mockLogger.AssertCalled(t, "Debug", mock.MatchedBy(func(msg string) bool {
		return msg == "Field populated"
	}), mock.Anything)
}

// TestDatabaseModuleInstanceAwareDSN tests field tracking with instance-aware database configuration
func TestDatabaseModuleInstanceAwareDSN(t *testing.T) {
	// Clear any environment variables from previous tests
	clearTestEnvironment(t)

	// Set up instance-aware database configuration
	// This simulates the user's specific scenario
	setEnvVarsWithCleanup(t, map[string]string{
		"TEST2_DB_PRIMARY_DRIVER":      "postgres",
		"TEST2_DB_PRIMARY_DSN":         "postgres://localhost/primary",
		"TEST2_DB_PRIMARY_MAX_CONNS":   "10",
		"TEST2_DB_SECONDARY_DRIVER":    "mysql",
		"TEST2_DB_SECONDARY_DSN":       "mysql://localhost/secondary",
		"TEST2_DB_SECONDARY_MAX_CONNS": "5",
	})

	cfg, tracker, mockLogger := createTestConfig()
	// Define database connection config (matching the Database module structure)
	type DBConnection struct {
		Driver   string `env:"DRIVER"`
		DSN      string `env:"DSN"`
		MaxConns int    `env:"MAX_CONNS"`
	}

	type DatabaseConfig struct {
		Connections map[string]DBConnection `yaml:"connections"`
	}

	config := &DatabaseConfig{
		Connections: map[string]DBConnection{
			"primary":   {},
			"secondary": {},
		},
	}

	// Add instance-aware environment feeder
	instanceFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "TEST2_DB_" + strings.ToUpper(instanceKey) + "_"
	})
	cfg.AddFeeder(instanceFeeder)

	// Add the configuration structure
	cfg.AddStructKey("database", config)

	// Feed configuration - this should use the regular Feed method first
	err := cfg.Feed()
	require.NoError(t, err)

	// Now use FeedInstances specifically for the connections map
	err = instanceFeeder.FeedInstances(config.Connections)
	require.NoError(t, err)

	// Debug: Check if field tracker has any populations
	populations := tracker.(*DefaultFieldTracker).FieldPopulations
	t.Logf("Field tracker has %d populations after feeding", len(populations))
	for i, pop := range populations {
		t.Logf("  %d: %s -> %v (from %s:%s, instance:%s)", i, pop.FieldPath, pop.Value, pop.SourceType, pop.SourceKey, pop.InstanceKey)
	}

	// Verify config was populated correctly
	require.Contains(t, config.Connections, "primary")
	require.Contains(t, config.Connections, "secondary")

	primaryConn := config.Connections["primary"]
	assert.Equal(t, "postgres", primaryConn.Driver)
	assert.Equal(t, "postgres://localhost/primary", primaryConn.DSN)
	assert.Equal(t, 10, primaryConn.MaxConns)

	secondaryConn := config.Connections["secondary"]
	assert.Equal(t, "mysql", secondaryConn.Driver)
	assert.Equal(t, "mysql://localhost/secondary", secondaryConn.DSN)
	assert.Equal(t, 5, secondaryConn.MaxConns)

	// Verify field tracking captured all field populations with instance awareness
	require.GreaterOrEqual(t, len(populations), 6, "Should track at least 6 field populations (3 fields × 2 instances)")

	// Verify primary instance DSN field tracking
	primaryDSNPop := findInstanceFieldPopulation(populations, "DSN", "primary")
	require.NotNil(t, primaryDSNPop, "Primary DSN field population should be tracked")
	assert.Equal(t, "postgres://localhost/primary", primaryDSNPop.Value)
	assert.Equal(t, "env", primaryDSNPop.SourceType)
	assert.Equal(t, "TEST2_DB_PRIMARY_DSN", primaryDSNPop.SourceKey)
	assert.Equal(t, "primary", primaryDSNPop.InstanceKey)
	assert.Contains(t, primaryDSNPop.SearchKeys, "TEST2_DB_PRIMARY_DSN")

	// Verify secondary instance DSN field tracking
	secondaryDSNPop := findInstanceFieldPopulation(populations, "DSN", "secondary")
	require.NotNil(t, secondaryDSNPop, "Secondary DSN field population should be tracked")
	assert.Equal(t, "mysql://localhost/secondary", secondaryDSNPop.Value)
	assert.Equal(t, "env", secondaryDSNPop.SourceType)
	assert.Equal(t, "TEST2_DB_SECONDARY_DSN", secondaryDSNPop.SourceKey)
	assert.Equal(t, "secondary", secondaryDSNPop.InstanceKey)
	assert.Contains(t, secondaryDSNPop.SearchKeys, "TEST2_DB_SECONDARY_DSN")

	// Verify that field tracking was used and logged
	mockLogger.AssertCalled(t, "Debug", mock.MatchedBy(func(msg string) bool {
		return msg == "Field populated"
	}), mock.Anything)
}

// TestMixedFeedersYamlAndEnv tests field tracking with mixed YAML and environment feeders
func TestMixedFeedersYamlAndEnv(t *testing.T) {
	// Clear any environment variables from previous tests
	clearTestEnvironment(t)

	// Only set the environment variables we want for this test
	// Others should be clean due to automatic cleanup from previous tests
	setEnvVarsWithCleanup(t, map[string]string{
		"TEST3_APP_NAME":       "Test App from ENV",
		"TEST3_DB_PRIMARY_DSN": "postgres://env/primary",
	})

	cfg, tracker, mockLogger := createTestConfig()

	// Create a temporary YAML file
	yamlContent := `
app:
  name: "Test App from YAML"
  debug: true
  port: 9090
database:
  connections:
    primary:
      driver: "postgres"
      max_conns: 20
    secondary:
      driver: "mysql"
      dsn: "mysql://yaml/secondary"
`
	yamlFile, err := os.CreateTemp("", "test_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(yamlFile.Name())

	_, err = yamlFile.WriteString(yamlContent)
	require.NoError(t, err)
	yamlFile.Close()

	// Define config structures
	type AppConfig struct {
		AppName string `env:"TEST3_APP_NAME" yaml:"name"`
		Debug   bool   `env:"TEST3_APP_DEBUG" yaml:"debug"`
		Port    int    `env:"TEST3_APP_PORT" yaml:"port"`
	}

	type DBConnection struct {
		Driver   string `env:"DRIVER" yaml:"driver"`
		DSN      string `env:"DSN" yaml:"dsn"`
		MaxConns int    `env:"MAX_CONNS" yaml:"max_conns"`
	}

	type DatabaseConfig struct {
		Connections map[string]DBConnection `yaml:"connections"`
	}

	type RootConfig struct {
		App      AppConfig      `yaml:"app"`
		Database DatabaseConfig `yaml:"database"`
	}

	config := &RootConfig{
		Database: DatabaseConfig{
			Connections: map[string]DBConnection{
				"primary":   {},
				"secondary": {},
			},
		},
	}

	// Add YAML feeder first (lower priority)
	yamlFeeder := feeders.NewYamlFeeder(yamlFile.Name())
	cfg.AddFeeder(yamlFeeder)

	// Add environment feeder (higher priority)
	envFeeder := feeders.NewEnvFeeder()
	cfg.AddFeeder(envFeeder)

	// Add instance-aware environment feeder (highest priority for instance fields)
	instanceFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "TEST3_DB_" + strings.ToUpper(instanceKey) + "_"
	})
	cfg.AddFeeder(instanceFeeder)

	// Add the configuration structure
	cfg.AddStructKey("root", config)

	// Feed configuration
	err = cfg.Feed()
	require.NoError(t, err)

	// Now use FeedInstances specifically for the connections map with instance-aware env
	t.Logf("Before FeedInstances: primary DSN = %s", config.Database.Connections["primary"].DSN)
	err = instanceFeeder.FeedInstances(config.Database.Connections)
	require.NoError(t, err)
	t.Logf("After FeedInstances: primary DSN = %s", config.Database.Connections["primary"].DSN)

	// Debug: Check if field tracker has any populations
	populations := tracker.(*DefaultFieldTracker).FieldPopulations
	t.Logf("Field tracker has %d populations after feeding", len(populations))
	for i, pop := range populations {
		t.Logf("  %d: %s -> %v (from %s:%s)", i, pop.FieldPath, pop.Value, pop.SourceType, pop.SourceKey)
	}

	// Verify config was populated from correct sources
	// APP_NAME should come from ENV (overriding YAML)
	assert.Equal(t, "Test App from ENV", config.App.AppName)
	// Debug should come from YAML (no ENV override)
	assert.Equal(t, true, config.App.Debug)
	// Port should come from YAML
	assert.Equal(t, 9090, config.App.Port)

	// Primary DSN should come from ENV (set via instance-aware feeder)
	primaryConn := config.Database.Connections["primary"]
	assert.Equal(t, "postgres://env/primary", primaryConn.DSN)
	// Primary driver should come from YAML
	assert.Equal(t, "postgres", primaryConn.Driver)

	// Secondary DSN should come from YAML
	secondaryConn := config.Database.Connections["secondary"]
	assert.Equal(t, "mysql://yaml/secondary", secondaryConn.DSN)

	// Verify field tracking shows correct sources

	// AppName should be tracked as populated from ENV (last wins)
	appNamePop := findLastFieldPopulation(populations, "AppName")
	require.NotNil(t, appNamePop, "AppName field population should be tracked")
	assert.Equal(t, "Test App from ENV", appNamePop.Value)
	assert.Equal(t, "env", appNamePop.SourceType)

	// Debug should be tracked as populated from YAML
	debugPop := findFieldPopulation(populations, "Debug")
	require.NotNil(t, debugPop, "Debug field population should be tracked")
	assert.Equal(t, true, debugPop.Value)
	assert.Equal(t, "yaml", debugPop.SourceType)

	// Primary DSN should be tracked as populated from ENV with instance awareness
	primaryDSNPop := findInstanceFieldPopulation(populations, "DSN", "primary")
	require.NotNil(t, primaryDSNPop, "Primary DSN field population should be tracked")
	assert.Equal(t, "postgres://env/primary", primaryDSNPop.Value)
	assert.Equal(t, "env", primaryDSNPop.SourceType)
	assert.Equal(t, "primary", primaryDSNPop.InstanceKey)

	// Verify that field tracking was used and logged
	mockLogger.AssertCalled(t, "Debug", mock.MatchedBy(func(msg string) bool {
		return msg == "Field populated"
	}), mock.Anything)
}

// Helper function to find a field population by field name
func findFieldPopulation(populations []FieldPopulation, fieldName string) *FieldPopulation {
	for _, pop := range populations {
		if pop.FieldName == fieldName {
			return &pop
		}
	}
	return nil
}

// Helper function to find the last field population by field name (for override scenarios)
func findLastFieldPopulation(populations []FieldPopulation, fieldName string) *FieldPopulation {
	var result *FieldPopulation
	for _, pop := range populations {
		if pop.FieldName == fieldName && pop.Value != nil {
			result = &pop
		}
	}
	return result
}

// Helper function to find a field population by field name and instance key
func findInstanceFieldPopulation(populations []FieldPopulation, fieldName, instanceKey string) *FieldPopulation {
	for _, pop := range populations {
		if pop.FieldName == fieldName && pop.InstanceKey == instanceKey {
			return &pop
		}
	}
	return nil
}
