package database

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GoCodeAlone/modular"
)

// TestDatabaseModuleWithInstanceAwareConfiguration tests the module with instance-aware env configuration
func TestDatabaseModuleWithInstanceAwareConfiguration(t *testing.T) {
	// Clear environment
	clearTestEnvVars(t)

	// Set up environment variables for multiple database connections
	envVars := map[string]string{
		"MAIN_DRIVER": "sqlite",
		"MAIN_DSN":    ":memory:",

		"READONLY_DRIVER": "sqlite", 
		"READONLY_DSN":    ":memory:",

		"CACHE_DRIVER": "sqlite",
		"CACHE_DSN":    ":memory:",
	}

	for key, value := range envVars {
		err := os.Setenv(key, value)
		require.NoError(t, err)
	}

	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Create a mock application
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	app := modular.NewStdApplication(nil, logger)

	// Create database module and register it
	module := NewModule()
	err := module.RegisterConfig(app)
	require.NoError(t, err)

	// Register instance-aware feeder for database connections
	configProvider, err := app.GetConfigSection(module.Name())
	require.NoError(t, err)

	config, ok := configProvider.GetConfig().(*Config)
	require.True(t, ok, "Config should be of type *Config")

	// Set up connections manually for test
	config.Connections = map[string]ConnectionConfig{
		"main":     {},
		"readonly": {},
		"cache":    {},
	}

	// Apply instance-aware environment variable feeding
	feeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return instanceKey + "_"
	})

	err = feeder.FeedInstances(config.Connections)
	require.NoError(t, err)

	// Verify connections were configured from environment variables
	assert.Equal(t, "sqlite", config.Connections["main"].Driver)
	assert.Equal(t, ":memory:", config.Connections["main"].DSN)

	assert.Equal(t, "sqlite", config.Connections["readonly"].Driver)
	assert.Equal(t, ":memory:", config.Connections["readonly"].DSN)

	assert.Equal(t, "sqlite", config.Connections["cache"].Driver)
	assert.Equal(t, ":memory:", config.Connections["cache"].DSN)

	// Initialize the module
	err = module.Init(app)
	require.NoError(t, err)

	// Start the module
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Verify all connections are available
	connections := module.GetConnections()
	assert.Len(t, connections, 3)
	assert.Contains(t, connections, "main")
	assert.Contains(t, connections, "readonly")
	assert.Contains(t, connections, "cache")

	// Verify we can get each connection
	mainDB, exists := module.GetConnection("main")
	assert.True(t, exists)
	assert.NotNil(t, mainDB)

	readonlyDB, exists := module.GetConnection("readonly")
	assert.True(t, exists)
	assert.NotNil(t, readonlyDB)

	cacheDB, exists := module.GetConnection("cache")
	assert.True(t, exists)
	assert.NotNil(t, cacheDB)

	// Clean up
	err = module.Stop(ctx)
	require.NoError(t, err)
}

// TestInstanceAwareConfigurationIntegration tests integration with config system
func TestInstanceAwareConfigurationIntegration(t *testing.T) {
	// This test demonstrates how to use instance-aware configuration in practice
	// Clear environment
	clearTestEnvVars(t)

	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":   "sqlite",
		"DB_PRIMARY_DSN":      ":memory:",
		"DB_SECONDARY_DRIVER": "sqlite",
		"DB_SECONDARY_DSN":    ":memory:",
	}

	for key, value := range envVars {
		err := os.Setenv(key, value)
		require.NoError(t, err)
	}

	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Create configuration
	config := &Config{
		Default: "primary",
		Connections: map[string]ConnectionConfig{
			"primary":   {},
			"secondary": {},
		},
	}

	// Create instance-aware feeder with module prefix
	feeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	})

	// Feed the configuration
	err := feeder.FeedInstances(config.Connections)
	require.NoError(t, err)

	// Verify configuration
	assert.Equal(t, "sqlite", config.Connections["primary"].Driver)
	assert.Equal(t, ":memory:", config.Connections["primary"].DSN)

	assert.Equal(t, "sqlite", config.Connections["secondary"].Driver)
	assert.Equal(t, ":memory:", config.Connections["secondary"].DSN)
}