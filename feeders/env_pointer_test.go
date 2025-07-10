package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structures for environment variable pointer handling
type EnvTestConfig struct {
	// Simple fields that should work with env vars
	Name        string `env:"APP_NAME"`
	Port        int    `env:"APP_PORT"`
	EnableDebug bool   `env:"APP_DEBUG"`

	// Pointer fields
	DatabaseURL *string `env:"DATABASE_URL"`
	MaxRetries  *int    `env:"MAX_RETRIES"`
	UseCache    *bool   `env:"USE_CACHE"`

	// Nested struct (env vars typically use dot notation or prefixes for these)
	Database EnvDatabaseConfig `env:"DATABASE"`
}

type EnvDatabaseConfig struct {
	Host     string `env:"DATABASE_HOST"`
	Port     int    `env:"DATABASE_PORT"`
	Username string `env:"DATABASE_USER"`
	Password string `env:"DATABASE_PASS"`
}

// Test basic env feeder with pointer fields
func TestEnvFeederWithPointers(t *testing.T) {
	// Clean up any existing env vars to avoid pollution from other tests
	cleanupEnvVars := []string{
		"APP_NAME", "APP_PORT", "APP_DEBUG",
		"DATABASE_URL", "MAX_RETRIES", "USE_CACHE",
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_USER", "DATABASE_PASS",
	}

	for _, envVar := range cleanupEnvVars {
		os.Unsetenv(envVar)
	}

	// Set up environment variables using t.Setenv for automatic cleanup
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("APP_PORT", "8080")
	t.Setenv("APP_DEBUG", "true")
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/testdb")
	t.Setenv("MAX_RETRIES", "5")
	t.Setenv("USE_CACHE", "false")
	t.Setenv("DATABASE_HOST", "localhost")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_USER", "testuser")
	t.Setenv("DATABASE_PASS", "testpass")

	// Test env feeder
	feeder := NewEnvFeeder()
	config := &EnvTestConfig{}

	err := feeder.Feed(config)
	require.NoError(t, err, "Env feeder should handle pointer fields without errors")

	// Verify basic fields
	assert.Equal(t, "test-app", config.Name)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.EnableDebug)

	// Verify pointer fields
	require.NotNil(t, config.DatabaseURL)
	assert.Equal(t, "postgres://localhost:5432/testdb", *config.DatabaseURL)

	require.NotNil(t, config.MaxRetries)
	assert.Equal(t, 5, *config.MaxRetries)

	require.NotNil(t, config.UseCache)
	assert.False(t, *config.UseCache)

	// Verify nested struct fields
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
	assert.Equal(t, "testuser", config.Database.Username)
	assert.Equal(t, "testpass", config.Database.Password)
}

func TestEnvFeederWithMissingPointerValues(t *testing.T) {
	// Reset global env catalog to avoid pollution from other tests
	ResetGlobalEnvCatalog()

	// Clean up any existing env vars to avoid pollution from other tests
	cleanupEnvVars := []string{
		"APP_NAME", "APP_PORT", "APP_DEBUG",
		"DATABASE_URL", "MAX_RETRIES", "USE_CACHE",
	}

	for _, envVar := range cleanupEnvVars {
		os.Unsetenv(envVar)
	}

	// Set only some environment variables using t.Setenv for automatic cleanup
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("APP_PORT", "8080")
	// Leave DATABASE_URL, MAX_RETRIES, USE_CACHE truly unset (don't call t.Setenv for them)

	// Test env feeder
	feeder := NewEnvFeeder()
	config := &EnvTestConfig{}

	err := feeder.Feed(config)
	require.NoError(t, err, "Env feeder should handle missing pointer values gracefully")

	// Verify set fields
	assert.Equal(t, "test-app", config.Name)
	assert.Equal(t, 8080, config.Port)

	// Verify pointer fields remain nil when env vars are missing or empty
	assert.Nil(t, config.DatabaseURL)
	assert.Nil(t, config.MaxRetries)
	assert.Nil(t, config.UseCache)
}

func TestEnvFeederWithNestedPointerStructs(t *testing.T) {
	// Test config with pointer to nested struct
	type NestedPointerConfig struct {
		Name     string             `env:"APP_NAME"`
		Database *EnvDatabaseConfig `env:"DATABASE"`
	}

	// Clean up any existing env vars to avoid pollution from other tests
	cleanupEnvVars := []string{
		"APP_NAME",
		"DATABASE_HOST", "DATABASE_PORT", "DATABASE_USER", "DATABASE_PASS",
	}

	for _, envVar := range cleanupEnvVars {
		os.Unsetenv(envVar)
	}

	// Set up environment variables using t.Setenv for automatic cleanup
	t.Setenv("APP_NAME", "nested-test")
	t.Setenv("DATABASE_HOST", "localhost")
	t.Setenv("DATABASE_PORT", "5432")
	t.Setenv("DATABASE_USER", "testuser")
	t.Setenv("DATABASE_PASS", "testpass")

	// Test env feeder
	feeder := NewEnvFeeder()
	config := &NestedPointerConfig{}

	err := feeder.Feed(config)
	require.NoError(t, err, "Env feeder should handle nested pointer structs")

	// Verify basic field
	assert.Equal(t, "nested-test", config.Name)

	// Note: Environment feeders typically don't automatically create nested struct pointers
	// from individual field environment variables. This would require more complex logic
	// or the use of instance-aware feeders with specific prefixes.
	// The test primarily ensures the feeder doesn't crash when encountering pointer struct fields.
}
