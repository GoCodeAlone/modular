package feeders

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceAwareEnvFeeder tests the new instance-aware environment variable feeder
func TestInstanceAwareEnvFeeder(t *testing.T) {
	// Sample config structure for testing multiple database connections
	type DatabaseConnectionConfig struct {
		Driver   string `env:"DRIVER"`
		DSN      string `env:"DSN"`
		User     string `env:"USER"`
		Password string `env:"PASSWORD"`
	}

	type MultiDatabaseConfig struct {
		Connections map[string]DatabaseConnectionConfig `json:"connections" yaml:"connections"`
	}

	tests := []struct {
		name           string
		envVars        map[string]string
		expectedConfig MultiDatabaseConfig
		instancePrefix func(instanceKey string) string
	}{
		{
			name: "multiple_database_connections_with_instance_prefixes",
			envVars: map[string]string{
				"MAIN_DRIVER":   "postgres",
				"MAIN_DSN":      "postgres://localhost/main",
				"MAIN_USER":     "main_user",
				"MAIN_PASSWORD": "main_pass",

				"READONLY_DRIVER":   "mysql",
				"READONLY_DSN":      "mysql://localhost/readonly",
				"READONLY_USER":     "readonly_user",
				"READONLY_PASSWORD": "readonly_pass",

				"CACHE_DRIVER":   "redis",
				"CACHE_DSN":      "redis://localhost/cache",
				"CACHE_USER":     "cache_user",
				"CACHE_PASSWORD": "cache_pass",
			},
			instancePrefix: func(instanceKey string) string {
				return instanceKey + "_"
			},
			expectedConfig: MultiDatabaseConfig{
				Connections: map[string]DatabaseConnectionConfig{
					"main": {
						Driver:   "postgres",
						DSN:      "postgres://localhost/main",
						User:     "main_user",
						Password: "main_pass",
					},
					"readonly": {
						Driver:   "mysql",
						DSN:      "mysql://localhost/readonly",
						User:     "readonly_user",
						Password: "readonly_pass",
					},
					"cache": {
						Driver:   "redis",
						DSN:      "redis://localhost/cache",
						User:     "cache_user",
						Password: "cache_pass",
					},
				},
			},
		},
		{
			name: "module_and_instance_prefixes",
			envVars: map[string]string{
				"DB_MAIN_DRIVER":   "postgres",
				"DB_MAIN_DSN":      "postgres://localhost/main",
				"DB_BACKUP_DRIVER": "postgres",
				"DB_BACKUP_DSN":    "postgres://localhost/backup",
			},
			instancePrefix: func(instanceKey string) string {
				return "DB_" + instanceKey + "_"
			},
			expectedConfig: MultiDatabaseConfig{
				Connections: map[string]DatabaseConnectionConfig{
					"main": {
						Driver: "postgres",
						DSN:    "postgres://localhost/main",
					},
					"backup": {
						Driver: "postgres",
						DSN:    "postgres://localhost/backup",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearTestEnv(t)

			// Set up environment variables
			for key, value := range tt.envVars {
				err := os.Setenv(key, value)
				require.NoError(t, err)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Create config with connections
			config := &MultiDatabaseConfig{
				Connections: make(map[string]DatabaseConnectionConfig),
			}

			// Add connection instances
			for instanceKey := range tt.expectedConfig.Connections {
				config.Connections[instanceKey] = DatabaseConnectionConfig{}
			}

			// Create and use the instance-aware feeder
			feeder := NewInstanceAwareEnvFeeder(tt.instancePrefix)
			err := feeder.FeedInstances(config.Connections)
			require.NoError(t, err)

			// Verify the configuration was populated correctly
			assert.Equal(t, tt.expectedConfig, *config)
		})
	}
}

// TestInstanceAwareEnvFeederWithSingleInstance tests backward compatibility
func TestInstanceAwareEnvFeederWithSingleInstance(t *testing.T) {
	type Config struct {
		Host string `env:"HOST"`
		Port int    `env:"PORT"`
	}

	// Set up environment
	clearTestEnv(t)
	err := os.Setenv("HOST", "localhost")
	require.NoError(t, err)
	err = os.Setenv("PORT", "8080")
	require.NoError(t, err)

	defer func() {
		os.Unsetenv("HOST")
		os.Unsetenv("PORT")
	}()

	config := &Config{}
	feeder := NewInstanceAwareEnvFeeder(nil) // No prefix function for single instance
	err = feeder.Feed(config)
	require.NoError(t, err)

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
}

// TestInstanceAwareEnvFeederErrors tests error conditions
func TestInstanceAwareEnvFeederErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectError bool
	}{
		{
			name:        "nil_input",
			input:       nil,
			expectError: true,
		},
		{
			name:        "non_pointer_input",
			input:       struct{}{},
			expectError: true,
		},
		{
			name:        "non_struct_pointer",
			input:       new(string),
			expectError: true,
		},
		{
			name:        "valid_struct_pointer",
			input:       &struct{ Field string `env:"FIELD"` }{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feeder := NewInstanceAwareEnvFeeder(nil)
			err := feeder.Feed(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestInstanceAwareEnvFeederComplexFeeder tests the ComplexFeeder interface
func TestInstanceAwareEnvFeederComplexFeeder(t *testing.T) {
	type ConnectionConfig struct {
		Host string `env:"HOST"`
		Port int    `env:"PORT"`
	}

	// Set up environment for database instance
	clearTestEnv(t)
	err := os.Setenv("DATABASE_HOST", "db.example.com")
	require.NoError(t, err)
	err = os.Setenv("DATABASE_PORT", "5432")
	require.NoError(t, err)

	defer func() {
		os.Unsetenv("DATABASE_HOST")
		os.Unsetenv("DATABASE_PORT")
	}()

	config := &ConnectionConfig{}
	feeder := NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return instanceKey + "_"
	})

	// Test FeedKey method (ComplexFeeder interface)
	err = feeder.FeedKey("database", config)
	require.NoError(t, err)

	assert.Equal(t, "db.example.com", config.Host)
	assert.Equal(t, 5432, config.Port)
}

// clearTestEnv clears relevant test environment variables
func clearTestEnv(t *testing.T) {
	envVars := []string{
		"DRIVER", "DSN", "USER", "PASSWORD", "HOST", "PORT",
		"MAIN_DRIVER", "MAIN_DSN", "MAIN_USER", "MAIN_PASSWORD",
		"READONLY_DRIVER", "READONLY_DSN", "READONLY_USER", "READONLY_PASSWORD",
		"CACHE_DRIVER", "CACHE_DSN", "CACHE_USER", "CACHE_PASSWORD",
		"DB_MAIN_DRIVER", "DB_MAIN_DSN", "DB_BACKUP_DRIVER", "DB_BACKUP_DSN",
		"DATABASE_HOST", "DATABASE_PORT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}