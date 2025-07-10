package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for TOML pointer map functionality
type TOMLTestConfig struct {
	Connections map[string]*TOMLTestConnection `toml:"connections"`
	Name        string                         `toml:"name"`
}

type TOMLTestConnection struct {
	Driver string `toml:"driver"`
	DSN    string `toml:"dsn"`
	Port   int    `toml:"port"`
}

func TestTOMLFeederMapWithPointers(t *testing.T) {
	// Create a temporary TOML file that mimics the database config structure
	tomlContent := `
name = "test-config"

[connections.primary]
driver = "postgres"
dsn = "postgres://localhost:5432/primary"
port = 5432

[connections.secondary]
driver = "mysql"
dsn = "mysql://localhost:3306/secondary"
port = 3306
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-toml-*.toml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(tomlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with TOML feeder
	feeder := NewTomlFeeder(tmpFile.Name())

	// Create test config
	config := &TOMLTestConfig{}

	// This should work without errors
	err = feeder.Feed(config)
	require.NoError(t, err, "TOML feeder should handle map[string]*Struct without errors")

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

func TestTOMLFeederMapWithValues(t *testing.T) {
	// Test with values instead of pointers
	type TOMLValueConfig struct {
		Connections map[string]TOMLTestConnection `toml:"connections"`
		Name        string                        `toml:"name"`
	}

	tomlContent := `
name = "value-config"

[connections.main]
driver = "sqlite"
dsn = ":memory:"
port = 0
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-toml-values-*.toml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(tomlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with TOML feeder
	feeder := NewTomlFeeder(tmpFile.Name())
	config := &TOMLValueConfig{}

	err = feeder.Feed(config)
	require.NoError(t, err, "TOML feeder should handle map[string]Struct without errors")

	// Verify the configuration was loaded correctly
	assert.Equal(t, "value-config", config.Name)
	require.NotNil(t, config.Connections)
	assert.Len(t, config.Connections, 1)

	// Check main connection
	require.Contains(t, config.Connections, "main")
	main := config.Connections["main"]
	assert.Equal(t, "sqlite", main.Driver)
	assert.Equal(t, ":memory:", main.DSN)
	assert.Equal(t, 0, main.Port)
}

func TestTOMLFeederComplexStructure(t *testing.T) {
	// Test more complex TOML structure
	type TOMLComplexConnection struct {
		Driver   string            `toml:"driver"`
		DSN      string            `toml:"dsn"`
		Port     int               `toml:"port"`
		Settings map[string]string `toml:"settings"`
	}

	type TOMLComplexConfig struct {
		Connections map[string]*TOMLComplexConnection `toml:"connections"`
		Default     string                            `toml:"default"`
	}

	tomlContent := `
default = "primary"

[connections.primary]
driver = "postgres"
dsn = "postgres://localhost:5432/primary"
port = 5432

[connections.primary.settings]
max_connections = "100"
timeout = "30"

[connections.cache]
driver = "redis"
dsn = "redis://localhost:6379"
port = 6379

[connections.cache.settings]
max_idle = "10"
pool_size = "20"
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-toml-complex-*.toml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(tomlContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with TOML feeder
	feeder := NewTomlFeeder(tmpFile.Name())
	config := &TOMLComplexConfig{}

	err = feeder.Feed(config)
	require.NoError(t, err, "TOML feeder should handle complex nested structures")

	// Verify the configuration
	assert.Equal(t, "primary", config.Default)
	require.NotNil(t, config.Connections)
	assert.Len(t, config.Connections, 2)

	// Check primary connection
	primary := config.Connections["primary"]
	require.NotNil(t, primary)
	assert.Equal(t, "postgres", primary.Driver)
	assert.Equal(t, 5432, primary.Port)
	require.NotNil(t, primary.Settings)
	assert.Equal(t, "100", primary.Settings["max_connections"])
	assert.Equal(t, "30", primary.Settings["timeout"])

	// Check cache connection
	cache := config.Connections["cache"]
	require.NotNil(t, cache)
	assert.Equal(t, "redis", cache.Driver)
	assert.Equal(t, 6379, cache.Port)
	require.NotNil(t, cache.Settings)
	assert.Equal(t, "10", cache.Settings["max_idle"])
	assert.Equal(t, "20", cache.Settings["pool_size"])
}
