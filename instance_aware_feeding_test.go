package modular

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/CrisisTextLine/modular/feeders"
)

// TestInstanceAwareFeedingAfterYAML tests that instance-aware feeding works correctly
// after YAML configuration has been loaded. This test recreates the issue where
// instance-aware feeding was looking at the original empty config instead of the
// config that was populated by YAML feeders.
func TestInstanceAwareFeedingAfterYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		envVars     map[string]string
		expected    map[string]string
	}{
		{
			name: "database_connections_with_yaml_structure_and_env_overrides",
			yamlContent: `
database:
  default: "primary"
  connections:
    primary:
      driver: "postgres"
      dsn: "postgres://localhost:5432/defaultdb"
    secondary:
      driver: "mysql"
      dsn: "mysql://localhost:3306/defaultdb"
    cache:
      driver: "sqlite3"
      dsn: ":memory:"
`,
			envVars: map[string]string{
				"DB_PRIMARY_DRIVER":   "sqlite3",
				"DB_PRIMARY_DSN":      "./test_primary.db",
				"DB_SECONDARY_DRIVER": "sqlite3",
				"DB_SECONDARY_DSN":    "./test_secondary.db",
				"DB_CACHE_DRIVER":     "sqlite3", // Explicitly set to prevent contamination
				"DB_CACHE_DSN":        "./test_cache.db",
			},
			expected: map[string]string{
				"primary.driver":   "sqlite3",
				"primary.dsn":      "./test_primary.db",
				"secondary.driver": "sqlite3",
				"secondary.dsn":    "./test_secondary.db",
				"cache.driver":     "sqlite3",
				"cache.dsn":        "./test_cache.db",
			},
		},
		{
			name: "webapp_instances_with_yaml_and_env",
			yamlContent: `
webapp:
  default: "api"
  instances:
    api:
      port: 8080
      host: "localhost"
    admin:
      port: 8081
      host: "localhost"
`,
			envVars: map[string]string{
				"WEBAPP_API_PORT":   "9080",
				"WEBAPP_API_HOST":   "0.0.0.0",
				"WEBAPP_ADMIN_PORT": "9081",
			},
			expected: map[string]string{
				"api.port":   "9080",
				"api.host":   "0.0.0.0",
				"admin.port": "9081",
				"admin.host": "localhost", // Should keep YAML value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary YAML file
			tmpFile := createTempYAMLFile(t, tt.yamlContent)
			defer os.Remove(tmpFile)

			// Set environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Create test structures
			var dbConfig *TestDatabaseConfig
			var webConfig *TestWebappConfig

			if tt.name == "database_connections_with_yaml_structure_and_env_overrides" {
				dbConfig = &TestDatabaseConfig{
					Default:     "primary",
					Connections: make(map[string]*TestConnectionConfig),
				}
			} else {
				webConfig = &TestWebappConfig{
					Default:   "api",
					Instances: make(map[string]*TestWebappInstance),
				}
			}

			// Setup feeders
			originalFeeders := ConfigFeeders
			ConfigFeeders = []Feeder{
				feeders.NewYamlFeeder(tmpFile),
				feeders.NewEnvFeeder(),
			}
			defer func() { ConfigFeeders = originalFeeders }()

			// Create application
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			app := NewStdApplication(NewStdConfigProvider(&TestAppConfig{}), logger)

			// Register config section
			if dbConfig != nil {
				instancePrefixFunc := func(instanceKey string) string {
					return "DB_" + instanceKey + "_"
				}
				configProvider := NewInstanceAwareConfigProvider(dbConfig, instancePrefixFunc)
				app.RegisterConfigSection("database", configProvider)
			} else {
				instancePrefixFunc := func(instanceKey string) string {
					return "WEBAPP_" + instanceKey + "_"
				}
				configProvider := NewInstanceAwareConfigProvider(webConfig, instancePrefixFunc)
				app.RegisterConfigSection("webapp", configProvider)
			}

			// Initialize the application (this triggers config loading)
			if err := app.Init(); err != nil {
				t.Fatalf("Failed to initialize application: %v", err)
			}

			// Get the config section and validate the results
			var provider ConfigProvider
			var err error
			if dbConfig != nil {
				provider, err = app.GetConfigSection("database")
			} else {
				provider, err = app.GetConfigSection("webapp")
			}
			if err != nil {
				t.Fatalf("Failed to get config section: %v", err)
			}

			// Validate that instance-aware feeding worked
			if dbConfig != nil {
				testDatabaseInstanceAwareFeedingResults(t, provider, tt.expected)
			} else {
				testWebappInstanceAwareFeedingResults(t, provider, tt.expected)
			}
		})
	}
}

// TestInstanceAwareFeedingRegressionBug tests the specific bug that was fixed:
// instance-aware feeding was checking the original provider config instead of
// the config that was populated by YAML feeders.
func TestInstanceAwareFeedingRegressionBug(t *testing.T) {
	// Create YAML content with database connections
	yamlContent := `
database:
  default: "writer"
  connections:
    writer:
      driver: "postgres"
      dsn: "postgres://localhost:5432/defaultdb"
    reader:
      driver: "postgres"
      dsn: "postgres://localhost:5432/defaultdb"
`

	// Create temporary YAML file
	tmpFile := createTempYAMLFile(t, yamlContent)
	defer os.Remove(tmpFile)

	// Set environment variables for instance-aware feeding
	envVars := map[string]string{
		"DB_WRITER_DRIVER": "sqlite3",
		"DB_WRITER_DSN":    "./writer.db",
		"DB_READER_DRIVER": "sqlite3",
		"DB_READER_DSN":    "./reader.db",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create database config with empty connections (this simulates the bug)
	dbConfig := &TestDatabaseConfig{
		Default:     "writer",
		Connections: make(map[string]*TestConnectionConfig),
	}

	// Setup feeders
	originalFeeders := ConfigFeeders
	ConfigFeeders = []Feeder{
		feeders.NewYamlFeeder(tmpFile),
		feeders.NewEnvFeeder(),
	}
	defer func() { ConfigFeeders = originalFeeders }()

	// Create application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	app := NewStdApplication(NewStdConfigProvider(&TestAppConfig{}), logger)

	// Register database config section with instance-aware provider
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	configProvider := NewInstanceAwareConfigProvider(dbConfig, instancePrefixFunc)
	app.RegisterConfigSection("database", configProvider)

	// Initialize the application (this triggers config loading)
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the config section
	provider, err := app.GetConfigSection("database")
	if err != nil {
		t.Fatalf("Failed to get config section: %v", err)
	}

	// Validate that the fix works: instance-aware feeding should find the connections
	// that were loaded from YAML and apply environment variable overrides
	iaProvider, ok := provider.(*InstanceAwareConfigProvider)
	if !ok {
		t.Fatalf("Expected InstanceAwareConfigProvider, got %T", provider)
	}

	finalConfig, ok := iaProvider.GetConfig().(*TestDatabaseConfig)
	if !ok {
		t.Fatalf("Expected TestDatabaseConfig, got %T", iaProvider.GetConfig())
	}

	// Validate that connections were loaded from YAML
	if len(finalConfig.Connections) == 0 {
		t.Fatal("REGRESSION: No connections found - YAML feeding failed")
	}

	// Validate that environment variable overrides were applied
	if writerConn, exists := finalConfig.Connections["writer"]; exists {
		if writerConn.Driver != "sqlite3" {
			t.Errorf("REGRESSION: Writer driver should be 'sqlite3' from env var, got '%s'", writerConn.Driver)
		}
		if writerConn.DSN != "./writer.db" {
			t.Errorf("REGRESSION: Writer DSN should be './writer.db' from env var, got '%s'", writerConn.DSN)
		}
	} else {
		t.Error("REGRESSION: Writer connection not found")
	}

	if readerConn, exists := finalConfig.Connections["reader"]; exists {
		if readerConn.Driver != "sqlite3" {
			t.Errorf("REGRESSION: Reader driver should be 'sqlite3' from env var, got '%s'", readerConn.Driver)
		}
		if readerConn.DSN != "./reader.db" {
			t.Errorf("REGRESSION: Reader DSN should be './reader.db' from env var, got '%s'", readerConn.DSN)
		}
	} else {
		t.Error("REGRESSION: Reader connection not found")
	}
}

// Test structures for instance-aware configuration testing
type TestDatabaseConfig struct {
	Default     string                           `yaml:"default"`
	Connections map[string]*TestConnectionConfig `yaml:"connections"`
}

func (c *TestDatabaseConfig) Validate() error {
	return nil
}

func (c *TestDatabaseConfig) GetInstanceConfigs() map[string]interface{} {
	instances := make(map[string]interface{})
	for name, connection := range c.Connections {
		instances[name] = connection
	}
	return instances
}

type TestConnectionConfig struct {
	Driver string `yaml:"driver" env:"DRIVER"`
	DSN    string `yaml:"dsn" env:"DSN"`
}

type TestWebappConfig struct {
	Default   string                         `yaml:"default"`
	Instances map[string]*TestWebappInstance `yaml:"instances"`
}

func (c *TestWebappConfig) Validate() error {
	return nil
}

func (c *TestWebappConfig) GetInstanceConfigs() map[string]interface{} {
	instances := make(map[string]interface{})
	for name, instance := range c.Instances {
		instances[name] = instance
	}
	return instances
}

type TestWebappInstance struct {
	Port int    `yaml:"port" env:"PORT"`
	Host string `yaml:"host" env:"HOST"`
}

type TestAppConfig struct {
	App TestAppSettings `yaml:"app"`
}

type TestAppSettings struct {
	Name string `yaml:"name" env:"APP_NAME" default:"Test App"`
}

func (c *TestAppConfig) Validate() error {
	return nil
}

func createTempYAMLFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return tmpFile.Name()
}

func testDatabaseInstanceAwareFeedingResults(t *testing.T, provider ConfigProvider, expected map[string]string) {
	iaProvider, ok := provider.(*InstanceAwareConfigProvider)
	if !ok {
		t.Fatalf("Expected InstanceAwareConfigProvider, got %T", provider)
	}

	config, ok := iaProvider.GetConfig().(*TestDatabaseConfig)
	if !ok {
		t.Fatalf("Expected TestDatabaseConfig, got %T", iaProvider.GetConfig())
	}

	// Validate each expected value
	for key, expectedValue := range expected {
		parts := splitKey(key)
		if len(parts) != 2 {
			t.Errorf("Invalid key format: %s", key)
			continue
		}

		connectionName := parts[0]
		fieldName := parts[1]

		connection, exists := config.Connections[connectionName]
		if !exists {
			t.Errorf("Connection '%s' not found", connectionName)
			continue
		}

		var actualValue string
		switch fieldName {
		case "driver":
			actualValue = connection.Driver
		case "dsn":
			actualValue = connection.DSN
		default:
			t.Errorf("Unknown field: %s", fieldName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Expected %s to be '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func testWebappInstanceAwareFeedingResults(t *testing.T, provider ConfigProvider, expected map[string]string) {
	iaProvider, ok := provider.(*InstanceAwareConfigProvider)
	if !ok {
		t.Fatalf("Expected InstanceAwareConfigProvider, got %T", provider)
	}

	config, ok := iaProvider.GetConfig().(*TestWebappConfig)
	if !ok {
		t.Fatalf("Expected TestWebappConfig, got %T", iaProvider.GetConfig())
	}

	// Validate each expected value
	for key, expectedValue := range expected {
		parts := splitKey(key)
		if len(parts) != 2 {
			t.Errorf("Invalid key format: %s", key)
			continue
		}

		instanceName := parts[0]
		fieldName := parts[1]

		instance, exists := config.Instances[instanceName]
		if !exists {
			t.Errorf("Instance '%s' not found", instanceName)
			continue
		}

		var actualValue string
		switch fieldName {
		case "port":
			actualValue = fmt.Sprintf("%d", instance.Port)
		case "host":
			actualValue = instance.Host
		default:
			t.Errorf("Unknown field: %s", fieldName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Expected %s to be '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func splitKey(key string) []string {
	parts := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		if dotIndex := findDotIndex(key); dotIndex != -1 {
			if i == 0 {
				parts = append(parts, key[:dotIndex])
				key = key[dotIndex+1:]
			} else {
				parts = append(parts, key)
			}
		} else {
			parts = append(parts, key)
			break
		}
	}
	return parts
}

func findDotIndex(s string) int {
	for i, c := range s {
		if c == '.' {
			return i
		}
	}
	return -1
}

// TestInstanceAwareFeedingOrderMatter tests that instance-aware feeding happens
// AFTER regular config feeding, not before. This ensures that the instances
// are available when the instance-aware feeding process runs.
func TestInstanceAwareFeedingOrderMatters(t *testing.T) {
	// Create YAML content
	yamlContent := `
test:
  default: "instance1"
  items:
    instance1:
      value: "yaml_value1"
    instance2:
      value: "yaml_value2"
`

	// Create temporary YAML file
	tmpFile := createTempYAMLFile(t, yamlContent)
	defer os.Remove(tmpFile)

	// Set environment variables
	t.Setenv("TEST_INSTANCE1_VALUE", "env_value1")
	t.Setenv("TEST_INSTANCE2_VALUE", "env_value2")

	// Create test config
	testConfig := &TestInstanceConfig{
		Default: "instance1",
		Items:   make(map[string]*TestInstanceItem),
	}

	// Setup feeders
	originalFeeders := ConfigFeeders
	ConfigFeeders = []Feeder{
		feeders.NewYamlFeeder(tmpFile),
		feeders.NewEnvFeeder(),
	}
	defer func() { ConfigFeeders = originalFeeders }()

	// Create application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	app := NewStdApplication(NewStdConfigProvider(&TestAppConfig{}), logger)

	// Register config section
	instancePrefixFunc := func(instanceKey string) string {
		return "TEST_" + instanceKey + "_"
	}
	configProvider := NewInstanceAwareConfigProvider(testConfig, instancePrefixFunc)
	app.RegisterConfigSection("test", configProvider)

	// Initialize the application
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the config section
	provider, err := app.GetConfigSection("test")
	if err != nil {
		t.Fatalf("Failed to get config section: %v", err)
	}

	// Validate results
	iaProvider, ok := provider.(*InstanceAwareConfigProvider)
	if !ok {
		t.Fatalf("Expected InstanceAwareConfigProvider, got %T", provider)
	}

	finalConfig, ok := iaProvider.GetConfig().(*TestInstanceConfig)
	if !ok {
		t.Fatalf("Expected TestInstanceConfig, got %T", iaProvider.GetConfig())
	}

	// Validate that YAML loaded the structure first
	if len(finalConfig.Items) == 0 {
		t.Fatal("YAML feeding failed - no items found")
	}

	// Validate that environment variables overrode YAML values
	if item1, exists := finalConfig.Items["instance1"]; exists {
		if item1.Value != "env_value1" {
			t.Errorf("Expected instance1.value to be 'env_value1', got '%s'", item1.Value)
		}
	} else {
		t.Error("instance1 not found")
	}

	if item2, exists := finalConfig.Items["instance2"]; exists {
		if item2.Value != "env_value2" {
			t.Errorf("Expected instance2.value to be 'env_value2', got '%s'", item2.Value)
		}
	} else {
		t.Error("instance2 not found")
	}
}

type TestInstanceConfig struct {
	Default string                       `yaml:"default"`
	Items   map[string]*TestInstanceItem `yaml:"items"`
}

func (c *TestInstanceConfig) Validate() error {
	return nil
}

func (c *TestInstanceConfig) GetInstanceConfigs() map[string]interface{} {
	instances := make(map[string]interface{})
	for name, item := range c.Items {
		instances[name] = item
	}
	return instances
}

type TestInstanceItem struct {
	Value string `yaml:"value" env:"VALUE"`
}

// TestInstanceAwareFeedingWithNoInstances tests that instance-aware feeding
// gracefully handles the case where no instances are defined in the config.
func TestInstanceAwareFeedingWithNoInstances(t *testing.T) {
	// Create YAML content with no instances
	yamlContent := `
test:
  default: "none"
  items: {}
`

	// Create temporary YAML file
	tmpFile := createTempYAMLFile(t, yamlContent)
	defer os.Remove(tmpFile)

	// Create test config
	testConfig := &TestInstanceConfig{
		Default: "none",
		Items:   make(map[string]*TestInstanceItem),
	}

	// Setup feeders
	originalFeeders := ConfigFeeders
	ConfigFeeders = []Feeder{
		feeders.NewYamlFeeder(tmpFile),
		feeders.NewEnvFeeder(),
	}
	defer func() { ConfigFeeders = originalFeeders }()

	// Create application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	app := NewStdApplication(NewStdConfigProvider(&TestAppConfig{}), logger)

	// Register config section
	instancePrefixFunc := func(instanceKey string) string {
		return "TEST_" + instanceKey + "_"
	}
	configProvider := NewInstanceAwareConfigProvider(testConfig, instancePrefixFunc)
	app.RegisterConfigSection("test", configProvider)

	// Initialize the application - this should not fail even with no instances
	if err := app.Init(); err != nil {
		t.Fatalf("Failed to initialize application: %v", err)
	}

	// Get the config section
	provider, err := app.GetConfigSection("test")
	if err != nil {
		t.Fatalf("Failed to get config section: %v", err)
	}

	// Validate results
	iaProvider, ok := provider.(*InstanceAwareConfigProvider)
	if !ok {
		t.Fatalf("Expected InstanceAwareConfigProvider, got %T", provider)
	}

	finalConfig, ok := iaProvider.GetConfig().(*TestInstanceConfig)
	if !ok {
		t.Fatalf("Expected TestInstanceConfig, got %T", iaProvider.GetConfig())
	}

	// Validate that empty instances are handled gracefully
	if len(finalConfig.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(finalConfig.Items))
	}

	if finalConfig.Default != "none" {
		t.Errorf("Expected default to be 'none', got '%s'", finalConfig.Default)
	}
}
