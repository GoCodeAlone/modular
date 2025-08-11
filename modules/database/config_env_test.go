package database

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CrisisTextLine/modular"
)

// TestConnectionConfigEnvMapping tests environment variable mapping for database connections
func TestConnectionConfigEnvMapping(t *testing.T) {
	tests := []struct {
		name           string
		instanceKey    string
		envVars        map[string]string
		expectedConfig ConnectionConfig
		instancePrefix func(instanceKey string) string
	}{
		{
			name:        "postgres_connection_with_env_vars",
			instanceKey: "main",
			envVars: map[string]string{
				"MAIN_DRIVER":                     "postgres",
				"MAIN_DSN":                        "postgres://user:pass@localhost:5432/maindb?sslmode=disable",
				"MAIN_MAX_OPEN_CONNECTIONS":       "25",
				"MAIN_MAX_IDLE_CONNECTIONS":       "5",
				"MAIN_CONNECTION_MAX_LIFETIME":    "3600s",
				"MAIN_CONNECTION_MAX_IDLE_TIME":   "300s",
				"MAIN_AWS_IAM_AUTH_ENABLED":       "true",
				"MAIN_AWS_IAM_AUTH_REGION":        "us-west-2",
				"MAIN_AWS_IAM_AUTH_DB_USER":       "iam_user",
				"MAIN_AWS_IAM_AUTH_TOKEN_REFRESH": "600",
			},
			instancePrefix: func(instanceKey string) string {
				return instanceKey + "_"
			},
			expectedConfig: ConnectionConfig{
				Driver:                "postgres",
				DSN:                   "postgres://user:pass@localhost:5432/maindb?sslmode=disable",
				MaxOpenConnections:    25,
				MaxIdleConnections:    5,
				ConnectionMaxLifetime: 3600 * time.Second,
				ConnectionMaxIdleTime: 300 * time.Second,
				AWSIAMAuth: &AWSIAMAuthConfig{
					Enabled:              true,
					Region:               "us-west-2",
					DBUser:               "iam_user",
					TokenRefreshInterval: 600,
				},
			},
		},
		{
			name:        "mysql_connection_minimal_config",
			instanceKey: "backup",
			envVars: map[string]string{
				"BACKUP_DRIVER": "mysql",
				"BACKUP_DSN":    "mysql://backup:secret@backup-host:3306/backupdb",
			},
			instancePrefix: func(instanceKey string) string {
				return instanceKey + "_"
			},
			expectedConfig: ConnectionConfig{
				Driver: "mysql",
				DSN:    "mysql://backup:secret@backup-host:3306/backupdb",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearTestEnvVars(t)

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

			// Create config
			config := &ConnectionConfig{}

			// Initialize AWSIAMAuth if needed for this test
			if tt.expectedConfig.AWSIAMAuth != nil {
				config.AWSIAMAuth = &AWSIAMAuthConfig{}
			}

			// Create and use the instance-aware feeder
			feeder := modular.NewInstanceAwareEnvFeeder(tt.instancePrefix)

			err := feeder.FeedKey(tt.instanceKey, config)
			require.NoError(t, err)

			// Verify the configuration was populated correctly
			assert.Equal(t, tt.expectedConfig, *config)
		})
	}
}

// TestMultipleDatabaseConnectionsWithEnvVars tests multiple database connections
func TestMultipleDatabaseConnectionsWithEnvVars(t *testing.T) {
	// Clear environment
	clearTestEnvVars(t)

	// Set up environment variables for multiple connections
	envVars := map[string]string{
		// Main database
		"MAIN_DRIVER": "postgres",
		"MAIN_DSN":    "postgres://localhost:5432/main",

		// Read-only replica
		"READONLY_DRIVER": "postgres",
		"READONLY_DSN":    "postgres://readonly:5432/main",

		// Cache database
		"CACHE_DRIVER": "redis",
		"CACHE_DSN":    "redis://localhost:6379/0",
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

	// Create configuration with multiple connections
	config := &Config{
		Default: "main",
		Connections: map[string]*ConnectionConfig{
			"main":     {},
			"readonly": {},
			"cache":    {},
		},
	}

	// Create instance-aware feeder
	feeder := modular.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return instanceKey + "_"
	})

	// Get instances and feed them (this is how the real application does it)
	instances := config.GetInstanceConfigs()
	for instanceKey, instanceConfig := range instances {
		err := feeder.FeedKey(instanceKey, instanceConfig)
		require.NoError(t, err)
	}

	// Verify each connection was configured correctly
	assert.Equal(t, "postgres", config.Connections["main"].Driver)
	assert.Equal(t, "postgres://localhost:5432/main", config.Connections["main"].DSN)

	assert.Equal(t, "postgres", config.Connections["readonly"].Driver)
	assert.Equal(t, "postgres://readonly:5432/main", config.Connections["readonly"].DSN)

	assert.Equal(t, "redis", config.Connections["cache"].Driver)
	assert.Equal(t, "redis://localhost:6379/0", config.Connections["cache"].DSN)
}

// TestDatabaseModuleWithInstanceAwareConfig tests the database module with instance-aware configuration
func TestDatabaseModuleWithInstanceAwareConfig(t *testing.T) {
	// This test will be implemented after we update the module to support instance-aware configuration
	t.Skip("Will be implemented after module update")
}

// clearTestEnvVars clears test environment variables
func clearTestEnvVars(_ *testing.T) {
	envVars := []string{
		"DRIVER", "DSN", "MAX_OPEN_CONNECTIONS", "MAX_IDLE_CONNECTIONS",
		"CONNECTION_MAX_LIFETIME", "CONNECTION_MAX_IDLE_TIME",
		"AWS_IAM_AUTH_ENABLED", "AWS_IAM_AUTH_REGION", "AWS_IAM_AUTH_DB_USER", "AWS_IAM_AUTH_TOKEN_REFRESH",
		"MAIN_DRIVER", "MAIN_DSN", "MAIN_MAX_OPEN_CONNECTIONS", "MAIN_MAX_IDLE_CONNECTIONS",
		"MAIN_CONNECTION_MAX_LIFETIME", "MAIN_CONNECTION_MAX_IDLE_TIME",
		"MAIN_AWS_IAM_AUTH_ENABLED", "MAIN_AWS_IAM_AUTH_REGION", "MAIN_AWS_IAM_AUTH_DB_USER", "MAIN_AWS_IAM_AUTH_TOKEN_REFRESH",
		"BACKUP_DRIVER", "BACKUP_DSN",
		"READONLY_DRIVER", "READONLY_DSN",
		"CACHE_DRIVER", "CACHE_DSN",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
