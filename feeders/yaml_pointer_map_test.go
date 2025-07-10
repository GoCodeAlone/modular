package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures to reproduce the YAML pointer map issue
type TestConfig struct {
	Connections map[string]*TestConnection `yaml:"connections"`
	Name        string                     `yaml:"name"`
}

type TestConnection struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
	Port   int    `yaml:"port"`
}

func TestYamlFeederMapWithPointers(t *testing.T) {
	// Create a temporary YAML file that mimics the database config structure
	yamlContent := `
name: "test-config"
connections:
  primary:
    driver: "postgres"
    dsn: "postgres://localhost:5432/primary"
    port: 5432
  secondary:
    driver: "mysql"
    dsn: "mysql://localhost:3306/secondary"
    port: 3306
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-yaml-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with verbose debugging disabled to avoid complex logger setup
	feeder := NewYamlFeeder(tmpFile.Name())

	// Create test config
	config := &TestConfig{}

	// This should work without the "Cannot convert map value" error
	err = feeder.Feed(config)
	require.NoError(t, err, "YAML feeder should handle map[string]*Struct without errors")

	// Verify the configuration was loaded correctly
	assert.Equal(t, "test-config", config.Name)
	require.NotNil(t, config.Connections)
	assert.Len(t, config.Connections, 2)

	// Check primary connection
	require.Contains(t, config.Connections, "primary")
	primary := config.Connections["primary"]
	require.NotNil(t, primary)
	assert.Equal(t, "postgres", primary.Driver)
	assert.Equal(t, "postgres://localhost:5432/primary", primary.DSN)
	assert.Equal(t, 5432, primary.Port)

	// Check secondary connection
	require.Contains(t, config.Connections, "secondary")
	secondary := config.Connections["secondary"]
	require.NotNil(t, secondary)
	assert.Equal(t, "mysql", secondary.Driver)
	assert.Equal(t, "mysql://localhost:3306/secondary", secondary.DSN)
	assert.Equal(t, 3306, secondary.Port)
}

func TestYamlFeederMapWithPointersComprehensive(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectError bool
	}{
		{
			name: "valid_config_with_pointer_maps",
			yamlContent: `
name: "comprehensive-test"
connections:
  db1:
    driver: "sqlite"
    dsn: ":memory:"
    port: 0
  db2:
    driver: "postgres"
    dsn: "postgres://localhost/test"
    port: 5432
`,
			expectError: false,
		},
		{
			name: "empty_connections_map",
			yamlContent: `
name: "empty-test"
connections: {}
`,
			expectError: false,
		},
		{
			name: "no_connections_field",
			yamlContent: `
name: "no-connections"
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "test-yaml-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.yamlContent)
			require.NoError(t, err)
			tmpFile.Close()

			// Test feeding
			feeder := NewYamlFeeder(tmpFile.Name())
			config := &TestConfig{}

			err = feeder.Feed(config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				// Basic validation that something was loaded
				assert.NotEmpty(t, config.Name)
			}
		})
	}
}

// Test the actual database config structure to ensure it works
func TestYamlFeederWithActualDatabaseConfig(t *testing.T) {
	// Import the actual database config for testing
	type DatabaseConnection struct {
		Driver                string `yaml:"driver"`
		DSN                   string `yaml:"dsn"`
		MaxOpenConnections    int    `yaml:"max_open_connections"`
		MaxIdleConnections    int    `yaml:"max_idle_connections"`
		ConnectionMaxLifetime int    `yaml:"connection_max_lifetime"`
		ConnectionMaxIdleTime int    `yaml:"connection_max_idle_time"`
	}

	type DatabaseConfig struct {
		Connections map[string]*DatabaseConnection `yaml:"connections"`
		Default     string                         `yaml:"default"`
	}

	yamlContent := `
default: "primary"
connections:
  primary:
    driver: "postgres"
    dsn: "postgres://user:pass@localhost:5432/main"
    max_open_connections: 25
    max_idle_connections: 5
    connection_max_lifetime: 3600
    connection_max_idle_time: 300
  cache:
    driver: "sqlite3"
    dsn: ":memory:"
    max_open_connections: 10
    max_idle_connections: 2
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-db-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test feeding
	feeder := NewYamlFeeder(tmpFile.Name())
	config := &DatabaseConfig{}

	err = feeder.Feed(config)
	require.NoError(t, err, "YAML feeder should handle database config structure")

	// Verify the configuration
	assert.Equal(t, "primary", config.Default)
	require.NotNil(t, config.Connections)
	assert.Len(t, config.Connections, 2)

	// Check primary connection
	primary := config.Connections["primary"]
	require.NotNil(t, primary)
	assert.Equal(t, "postgres", primary.Driver)
	assert.Equal(t, "postgres://user:pass@localhost:5432/main", primary.DSN)
	assert.Equal(t, 25, primary.MaxOpenConnections)
	assert.Equal(t, 5, primary.MaxIdleConnections)

	// Check cache connection
	cache := config.Connections["cache"]
	require.NotNil(t, cache)
	assert.Equal(t, "sqlite3", cache.Driver)
	assert.Equal(t, ":memory:", cache.DSN)
	assert.Equal(t, 10, cache.MaxOpenConnections)
}
