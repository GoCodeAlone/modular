package modular

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvMappingForModules tests that module configurations can be populated from environment variables
func TestEnvMappingForModules(t *testing.T) {
	// Test various module configurations to ensure env mapping works
	t.Run("cache_config_env_mapping", func(t *testing.T) {
		type CacheConfig struct {
			Engine          string `env:"ENGINE"`
			DefaultTTL      int    `env:"DEFAULT_TTL"`
			CleanupInterval int    `env:"CLEANUP_INTERVAL"`
			MaxItems        int    `env:"MAX_ITEMS"`
			RedisURL        string `env:"REDIS_URL"`
			RedisPassword   string `env:"REDIS_PASSWORD"`
			RedisDB         int    `env:"REDIS_DB"`
		}

		// Clear environment
		envVars := []string{"ENGINE", "DEFAULT_TTL", "CLEANUP_INTERVAL", "MAX_ITEMS",
			"REDIS_URL", "REDIS_PASSWORD", "REDIS_DB"}
		for _, env := range envVars {
			os.Unsetenv(env)
		}

		// Set up environment variables
		testEnvVars := map[string]string{
			"ENGINE":           "redis",
			"DEFAULT_TTL":      "3600",
			"CLEANUP_INTERVAL": "300",
			"MAX_ITEMS":        "10000",
			"REDIS_URL":        "redis://localhost:6379",
			"REDIS_PASSWORD":   "secret123",
			"REDIS_DB":         "5",
		}

		for key, value := range testEnvVars {
			err := os.Setenv(key, value)
			require.NoError(t, err)
		}

		defer func() {
			for key := range testEnvVars {
				os.Unsetenv(key)
			}
		}()

		// Test regular env feeder
		config := &CacheConfig{}
		feeder := NewInstanceAwareEnvFeeder(nil) // Use as regular feeder
		err := feeder.Feed(config)
		require.NoError(t, err)

		// Verify configuration was populated
		assert.Equal(t, "redis", config.Engine)
		assert.Equal(t, 3600, config.DefaultTTL)
		assert.Equal(t, 300, config.CleanupInterval)
		assert.Equal(t, 10000, config.MaxItems)
		assert.Equal(t, "redis://localhost:6379", config.RedisURL)
		assert.Equal(t, "secret123", config.RedisPassword)
		assert.Equal(t, 5, config.RedisDB)
	})

	t.Run("httpserver_config_env_mapping", func(t *testing.T) {
		type HTTPServerConfig struct {
			Host            string `env:"HOST"`
			Port            int    `env:"PORT"`
			ReadTimeout     int    `env:"READ_TIMEOUT"`
			WriteTimeout    int    `env:"WRITE_TIMEOUT"`
			IdleTimeout     int    `env:"IDLE_TIMEOUT"`
			ShutdownTimeout int    `env:"SHUTDOWN_TIMEOUT"`
		}

		// Clear environment
		envVars := []string{"HOST", "PORT", "READ_TIMEOUT", "WRITE_TIMEOUT", "IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT"}
		for _, env := range envVars {
			os.Unsetenv(env)
		}

		// Set up environment variables
		testEnvVars := map[string]string{
			"HOST":             "localhost",
			"PORT":             "8080",
			"READ_TIMEOUT":     "30",
			"WRITE_TIMEOUT":    "30",
			"IDLE_TIMEOUT":     "60",
			"SHUTDOWN_TIMEOUT": "10",
		}

		for key, value := range testEnvVars {
			err := os.Setenv(key, value)
			require.NoError(t, err)
		}

		defer func() {
			for key := range testEnvVars {
				os.Unsetenv(key)
			}
		}()

		// Test regular env feeder
		config := &HTTPServerConfig{}
		feeder := NewInstanceAwareEnvFeeder(nil) // Use as regular feeder
		err := feeder.Feed(config)
		require.NoError(t, err)

		// Verify configuration was populated
		assert.Equal(t, "localhost", config.Host)
		assert.Equal(t, 8080, config.Port)
		assert.Equal(t, 30, config.ReadTimeout)
		assert.Equal(t, 30, config.WriteTimeout)
		assert.Equal(t, 60, config.IdleTimeout)
		assert.Equal(t, 10, config.ShutdownTimeout)
	})

	t.Run("instance_aware_httpserver_configs", func(t *testing.T) {
		type HTTPServerConfig struct {
			Host string `env:"HOST"`
			Port int    `env:"PORT"`
		}

		// Clear environment
		clearEnvVars := []string{"HOST", "PORT", "API_HOST", "API_PORT", "ADMIN_HOST", "ADMIN_PORT"}
		for _, env := range clearEnvVars {
			os.Unsetenv(env)
		}

		// Set up environment variables for multiple server instances
		testEnvVars := map[string]string{
			"API_HOST":   "api.example.com",
			"API_PORT":   "8080",
			"ADMIN_HOST": "admin.example.com",
			"ADMIN_PORT": "9090",
		}

		for key, value := range testEnvVars {
			err := os.Setenv(key, value)
			require.NoError(t, err)
		}

		defer func() {
			for key := range testEnvVars {
				os.Unsetenv(key)
			}
		}()

		// Test instance-aware configuration
		configs := map[string]HTTPServerConfig{
			"api":   {},
			"admin": {},
		}

		feeder := NewInstanceAwareEnvFeeder(func(instanceKey string) string {
			return instanceKey + "_"
		})

		err := feeder.FeedInstances(configs)
		require.NoError(t, err)

		// Verify each instance was configured correctly
		assert.Equal(t, "api.example.com", configs["api"].Host)
		assert.Equal(t, 8080, configs["api"].Port)

		assert.Equal(t, "admin.example.com", configs["admin"].Host)
		assert.Equal(t, 9090, configs["admin"].Port)
	})
}
