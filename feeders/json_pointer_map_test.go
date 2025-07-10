package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for JSON pointer map functionality
type JSONTestConfig struct {
	Connections map[string]*JSONTestConnection `json:"connections"`
	Name        string                         `json:"name"`
}

type JSONTestConnection struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
	Port   int    `json:"port"`
}

func TestJSONFeederMapWithPointers(t *testing.T) {
	// Create a temporary JSON file that mimics the database config structure
	jsonContent := `{
	"name": "test-config",
	"connections": {
		"primary": {
			"driver": "postgres",
			"dsn": "postgres://localhost:5432/primary",
			"port": 5432
		},
		"secondary": {
			"driver": "mysql",
			"dsn": "mysql://localhost:3306/secondary",
			"port": 3306
		}
	}
}`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-json-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with JSON feeder
	feeder := NewJSONFeeder(tmpFile.Name())

	// Create test config
	config := &JSONTestConfig{}

	// This should work without errors
	err = feeder.Feed(config)
	require.NoError(t, err, "JSON feeder should handle map[string]*Struct without errors")

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

func TestJSONFeederMapWithValues(t *testing.T) {
	// Test with values instead of pointers
	type JSONValueConfig struct {
		Connections map[string]JSONTestConnection `json:"connections"`
		Name        string                        `json:"name"`
	}

	jsonContent := `{
	"name": "value-config",
	"connections": {
		"main": {
			"driver": "sqlite",
			"dsn": ":memory:",
			"port": 0
		}
	}
}`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-json-values-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with JSON feeder
	feeder := NewJSONFeeder(tmpFile.Name())
	config := &JSONValueConfig{}

	err = feeder.Feed(config)
	require.NoError(t, err, "JSON feeder should handle map[string]Struct without errors")

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

func TestJSONFeederWithNilMapEntries(t *testing.T) {
	// Test handling of null entries in JSON
	jsonContent := `{
	"name": "nil-test",
	"connections": {
		"valid": {
			"driver": "postgres",
			"dsn": "postgres://localhost:5432/valid",
			"port": 5432
		},
		"invalid": null
	}
}`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-json-nil-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(jsonContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Test with JSON feeder
	feeder := NewJSONFeeder(tmpFile.Name())
	config := &JSONTestConfig{}

	err = feeder.Feed(config)
	require.NoError(t, err, "JSON feeder should handle null entries gracefully")

	// Verify the configuration
	assert.Equal(t, "nil-test", config.Name)
	require.NotNil(t, config.Connections)
	assert.Len(t, config.Connections, 2)

	// Check valid connection
	require.Contains(t, config.Connections, "valid")
	valid := config.Connections["valid"]
	require.NotNil(t, valid)
	assert.Equal(t, "postgres", valid.Driver)

	// Check invalid connection (should be nil)
	require.Contains(t, config.Connections, "invalid")
	invalid := config.Connections["invalid"]
	assert.Nil(t, invalid) // JSON null should result in nil pointer
}
