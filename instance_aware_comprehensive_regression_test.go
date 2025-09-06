package modular

import (
	"log/slog"
	"os"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
)

// TestInstanceAwareComprehensiveRegressionSuite creates a comprehensive test suite
// to ensure that instance-aware configuration works correctly in all scenarios
// and catches regressions where configuration modifications don't persist.
func TestInstanceAwareComprehensiveRegressionSuite(t *testing.T) {
	t.Run("PointerMapValues_FeedInstances", testPointerMapValuesFeedInstances)
	t.Run("PointerMapValues_GetInstanceConfigsAndFeedKey", testPointerMapValuesGetInstanceConfigsAndFeedKey)
	t.Run("NonPointerMapValues_FeedInstances", testNonPointerMapValuesFeedInstances)
	t.Run("MixedScenario_EndToEnd", testMixedScenarioEndToEnd)
	t.Run("RegressionDetection_CopyVsOriginal", testRegressionDetectionCopyVsOriginal)
}

// testPointerMapValuesFeedInstances tests that FeedInstances works with pointer map values
func testPointerMapValuesFeedInstances(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER": "sqlite3",
		"DB_PRIMARY_DSN":    "./primary_test.db",
		"DB_CACHE_DRIVER":   "redis",
		"DB_CACHE_DSN":      "redis://localhost:6379",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create config with pointer map values (like our fixed database config)
	config := &TestDatabaseConfig{
		Default: "primary",
		Connections: map[string]*TestConnectionConfig{
			"primary": {
				Driver: "postgres",
				DSN:    "postgres://localhost/original",
			},
			"cache": {
				Driver: "mysql",
				DSN:    "mysql://localhost/original",
			},
		},
	}

	// Store original pointers to verify they're modified
	originalPrimary := config.Connections["primary"]
	originalCache := config.Connections["cache"]

	// Create instance-aware feeder
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	feeder := feeders.NewInstanceAwareEnvFeeder(instancePrefixFunc)

	// Use FeedInstances directly (this should work with pointer values)
	err := feeder.FeedInstances(config.Connections)
	if err != nil {
		t.Fatalf("FeedInstances failed: %v", err)
	}

	// Verify that the ORIGINAL pointers were modified
	if originalPrimary.Driver != "sqlite3" {
		t.Errorf("Original primary pointer not modified: expected driver 'sqlite3', got '%s'", originalPrimary.Driver)
	}
	if originalPrimary.DSN != "./primary_test.db" {
		t.Errorf("Original primary pointer not modified: expected DSN './primary_test.db', got '%s'", originalPrimary.DSN)
	}

	if originalCache.Driver != "redis" {
		t.Errorf("Original cache pointer not modified: expected driver 'redis', got '%s'", originalCache.Driver)
	}
	if originalCache.DSN != "redis://localhost:6379" {
		t.Errorf("Original cache pointer not modified: expected DSN 'redis://localhost:6379', got '%s'", originalCache.DSN)
	}

	// Verify that accessing through the map gives the same (modified) values
	if config.Connections["primary"].Driver != "sqlite3" {
		t.Errorf("Map access shows different value: expected driver 'sqlite3', got '%s'", config.Connections["primary"].Driver)
	}

	// Verify that the pointers are still the same (not replaced)
	if config.Connections["primary"] != originalPrimary {
		t.Error("Pointer was replaced instead of modified - this would break references")
	}
	if config.Connections["cache"] != originalCache {
		t.Error("Pointer was replaced instead of modified - this would break references")
	}
}

// testPointerMapValuesGetInstanceConfigsAndFeedKey tests the GetInstanceConfigs + FeedKey approach
func testPointerMapValuesGetInstanceConfigsAndFeedKey(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER": "sqlite3",
		"DB_PRIMARY_DSN":    "./primary_feedkey.db",
		"DB_CACHE_DRIVER":   "redis",
		"DB_CACHE_DSN":      "redis://localhost:6379",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create config with pointer map values
	config := &TestDatabaseConfig{
		Default: "primary",
		Connections: map[string]*TestConnectionConfig{
			"primary": {
				Driver: "postgres",
				DSN:    "postgres://localhost/original",
			},
			"cache": {
				Driver: "mysql",
				DSN:    "mysql://localhost/original",
			},
		},
	}

	// Store original pointers to verify they're modified
	originalPrimary := config.Connections["primary"]
	originalCache := config.Connections["cache"]

	// Create instance-aware feeder
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	feeder := feeders.NewInstanceAwareEnvFeeder(instancePrefixFunc)

	// Use GetInstanceConfigs + FeedKey approach (this is how the framework does it)
	instances := config.GetInstanceConfigs()
	for instanceKey, instanceConfig := range instances {
		if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
			t.Fatalf("FeedKey failed for instance %s: %v", instanceKey, err)
		}
	}

	// Verify that the ORIGINAL pointers were modified
	if originalPrimary.Driver != "sqlite3" {
		t.Errorf("Original primary pointer not modified: expected driver 'sqlite3', got '%s'", originalPrimary.Driver)
	}
	if originalPrimary.DSN != "./primary_feedkey.db" {
		t.Errorf("Original primary pointer not modified: expected DSN './primary_feedkey.db', got '%s'", originalPrimary.DSN)
	}

	if originalCache.Driver != "redis" {
		t.Errorf("Original cache pointer not modified: expected driver 'redis', got '%s'", originalCache.Driver)
	}

	// Verify that the pointers are still the same (not replaced)
	if config.Connections["primary"] != originalPrimary {
		t.Error("Pointer was replaced instead of modified - this would break references")
	}
	if config.Connections["cache"] != originalCache {
		t.Error("Pointer was replaced instead of modified - this would break references")
	}
}

// testNonPointerMapValuesFeedInstances tests that FeedInstances still works with non-pointer map values
func testNonPointerMapValuesFeedInstances(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"WEBAPP_FRONTEND_PORT": "3000",
		"WEBAPP_FRONTEND_HOST": "localhost",
		"WEBAPP_API_PORT":      "8080",
		"WEBAPP_API_HOST":      "api.example.com",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create config with NON-pointer map values (for backward compatibility testing)
	type LegacyInstanceConfig struct {
		Port int    `env:"PORT"`
		Host string `env:"HOST"`
	}

	legacyConfigs := map[string]LegacyInstanceConfig{
		"frontend": {
			Port: 80,
			Host: "example.com",
		},
		"api": {
			Port: 443,
			Host: "oldapi.example.com",
		},
	}

	// Create instance-aware feeder
	instancePrefixFunc := func(instanceKey string) string {
		return "WEBAPP_" + instanceKey + "_"
	}
	feeder := feeders.NewInstanceAwareEnvFeeder(instancePrefixFunc)

	// Use FeedInstances with non-pointer values (should still work)
	err := feeder.FeedInstances(legacyConfigs)
	if err != nil {
		t.Fatalf("FeedInstances failed with non-pointer values: %v", err)
	}

	// Verify that the map was updated with environment variable values
	if legacyConfigs["frontend"].Port != 3000 {
		t.Errorf("Expected frontend port 3000, got %d", legacyConfigs["frontend"].Port)
	}
	if legacyConfigs["frontend"].Host != "localhost" {
		t.Errorf("Expected frontend host 'localhost', got '%s'", legacyConfigs["frontend"].Host)
	}

	if legacyConfigs["api"].Port != 8080 {
		t.Errorf("Expected api port 8080, got %d", legacyConfigs["api"].Port)
	}
	if legacyConfigs["api"].Host != "api.example.com" {
		t.Errorf("Expected api host 'api.example.com', got '%s'", legacyConfigs["api"].Host)
	}
}

// testMixedScenarioEndToEnd tests a full end-to-end scenario with the application framework
func testMixedScenarioEndToEnd(t *testing.T) {
	// Create YAML content
	yamlContent := `
database:
  default: "primary"
  connections:
    primary:
      driver: "postgres"
      dsn: "postgres://localhost:5432/yaml_db"
    secondary:
      driver: "mysql"  
      dsn: "mysql://localhost:3306/yaml_db"
`

	// Create temporary YAML file
	tmpFile := createTempYAMLFile(t, yamlContent)
	defer os.Remove(tmpFile)

	// Set environment variables that should override YAML values
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":   "sqlite3",
		"DB_PRIMARY_DSN":      "./end_to_end_primary.db",
		"DB_SECONDARY_DRIVER": "sqlite3",
		"DB_SECONDARY_DSN":    "./end_to_end_secondary.db",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Setup per-app feeders (avoid mutating global ConfigFeeders for parallel safety)
	feedersSlice := []Feeder{feeders.NewYamlFeeder(tmpFile), feeders.NewEnvFeeder()}

	// Create config with pointer semantics
	dbConfig := &TestDatabaseConfig{
		Default:     "",
		Connections: make(map[string]*TestConnectionConfig),
	}

	// Store original map reference to verify connections are populated in the same map
	originalMapEmpty := len(dbConfig.Connections) == 0

	// Create application with proper main config and logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	app := NewStdApplication(NewStdConfigProvider(&TestAppConfig{
		App: TestAppSettings{
			Name: "test-app",
		},
	}), logger).(*StdApplication)

	// Register database config section with instance-aware provider
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	configProvider := NewInstanceAwareConfigProvider(dbConfig, instancePrefixFunc)
	app.RegisterConfigSection("database", configProvider)

	// Apply per-app feeders before initialization
	app.SetConfigFeeders(feedersSlice)
	// Initialize the application (this should load YAML + apply instance-aware env overrides)
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Verify that the original map was populated (not replaced with a new one)
	if !originalMapEmpty {
		t.Error("Test setup error: map should have started empty")
	}

	// Verify that YAML structure was loaded
	if len(dbConfig.Connections) == 0 {
		t.Fatal("REGRESSION: No connections loaded from YAML")
	}

	if dbConfig.Default != "primary" {
		t.Errorf("REGRESSION: Default not loaded from YAML, expected 'primary', got '%s'", dbConfig.Default)
	}

	// Verify that environment variables overrode YAML values
	primaryConn, exists := dbConfig.Connections["primary"]
	if !exists {
		t.Fatal("REGRESSION: Primary connection not found")
	}

	if primaryConn.Driver != "sqlite3" {
		t.Errorf("REGRESSION: Environment override failed, expected driver 'sqlite3', got '%s'", primaryConn.Driver)
	}
	if primaryConn.DSN != "./end_to_end_primary.db" {
		t.Errorf("REGRESSION: Environment override failed, expected DSN './end_to_end_primary.db', got '%s'", primaryConn.DSN)
	}

	secondaryConn, exists := dbConfig.Connections["secondary"]
	if !exists {
		t.Fatal("REGRESSION: Secondary connection not found")
	}

	if secondaryConn.Driver != "sqlite3" {
		t.Errorf("REGRESSION: Environment override failed, expected driver 'sqlite3', got '%s'", secondaryConn.Driver)
	}
	if secondaryConn.DSN != "./end_to_end_secondary.db" {
		t.Errorf("REGRESSION: Environment override failed, expected DSN './end_to_end_secondary.db', got '%s'", secondaryConn.DSN)
	}
}

// testRegressionDetectionCopyVsOriginal specifically tests the scenario that would
// fail if someone reverted GetInstanceConfigs to return copies instead of pointers
func testRegressionDetectionCopyVsOriginal(t *testing.T) {
	t.Setenv("DB_TEST_DRIVER", "overridden_driver")
	t.Setenv("DB_TEST_DSN", "overridden_dsn")

	// Create a config where GetInstanceConfigs might return copies (the bug scenario)
	config := &TestDatabaseConfig{
		Default: "test",
		Connections: map[string]*TestConnectionConfig{
			"test": {
				Driver: "original_driver",
				DSN:    "original_dsn",
			},
		},
	}

	// Store reference to original connection
	originalConnection := config.Connections["test"]

	// Create a "broken" version of GetInstanceConfigs that returns copies
	// This simulates what would happen if someone reverted the fix
	brokenGetInstanceConfigs := func() map[string]interface{} {
		instances := make(map[string]interface{})
		for name, connection := range config.Connections {
			// BUG: Creating a copy instead of returning pointer to original
			connectionCopy := *connection
			instances[name] = &connectionCopy
		}
		return instances
	}

	// Test with the BROKEN version (should fail to modify original)
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	feeder := feeders.NewInstanceAwareEnvFeeder(instancePrefixFunc)

	brokenInstances := brokenGetInstanceConfigs()
	for instanceKey, instanceConfig := range brokenInstances {
		if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
			t.Fatalf("FeedKey failed: %v", err)
		}
	}

	// Verify that with the BROKEN version, the original is NOT modified
	if originalConnection.Driver == "overridden_driver" {
		t.Error("UNEXPECTED: Original was modified with broken GetInstanceConfigs - test setup is wrong")
	}
	if originalConnection.Driver != "original_driver" {
		t.Error("Original connection was unexpectedly modified")
	}

	// Now test with the CORRECT version (should modify original)
	correctInstances := config.GetInstanceConfigs()
	for instanceKey, instanceConfig := range correctInstances {
		if err := feeder.FeedKey(instanceKey, instanceConfig); err != nil {
			t.Fatalf("FeedKey failed: %v", err)
		}
	}

	// Verify that with the CORRECT version, the original IS modified
	if originalConnection.Driver != "overridden_driver" {
		t.Errorf("REGRESSION: Original was not modified with correct GetInstanceConfigs, expected 'overridden_driver', got '%s'", originalConnection.Driver)
	}
	if originalConnection.DSN != "overridden_dsn" {
		t.Errorf("REGRESSION: Original was not modified with correct GetInstanceConfigs, expected 'overridden_dsn', got '%s'", originalConnection.DSN)
	}

	// Verify that the config map still points to the same modified object
	if config.Connections["test"] != originalConnection {
		t.Error("REGRESSION: Map no longer points to original connection object")
	}
	if config.Connections["test"].Driver != "overridden_driver" {
		t.Errorf("REGRESSION: Map access shows wrong value, expected 'overridden_driver', got '%s'", config.Connections["test"].Driver)
	}
}
