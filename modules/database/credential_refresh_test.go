package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractDatabaseAndOptions tests the extraction of database name and options from DSN
func TestExtractDatabaseAndOptions(t *testing.T) {
	tests := []struct {
		name         string
		dsn          string
		expectedDB   string
		expectedOpts map[string]string
		expectError  bool
	}{
		{
			name:         "URL-style DSN with database",
			dsn:          "postgres://user:password@host:5432/mydb",
			expectedDB:   "mydb",
			expectedOpts: map[string]string{},
			expectError:  false,
		},
		{
			name:         "URL-style DSN with database and options",
			dsn:          "postgres://user:password@host:5432/mydb?sslmode=disable&connect_timeout=10",
			expectedDB:   "mydb",
			expectedOpts: map[string]string{"sslmode": "disable", "connect_timeout": "10"},
			expectError:  false,
		},
		{
			name:         "URL-style DSN without database",
			dsn:          "postgres://user:password@host:5432",
			expectedDB:   "",
			expectedOpts: map[string]string{},
			expectError:  false,
		},
		{
			name:         "Key-value style DSN with database",
			dsn:          "host=localhost port=5432 dbname=mydb sslmode=disable",
			expectedDB:   "mydb",
			expectedOpts: map[string]string{"sslmode": "disable"},
			expectError:  false,
		},
		{
			name:         "Key-value style DSN without database",
			dsn:          "host=localhost port=5432 sslmode=disable",
			expectedDB:   "",
			expectedOpts: map[string]string{"sslmode": "disable"},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, opts, err := extractDatabaseAndOptions(tt.dsn)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDB, db)
				assert.Equal(t, len(tt.expectedOpts), len(opts))
				for k, v := range tt.expectedOpts {
					assert.Equal(t, v, opts[k], "Option %s mismatch", k)
				}
			}
		})
	}
}

// TestDetermineDriverAndPort tests driver name and port determination
func TestDetermineDriverAndPort(t *testing.T) {
	tests := []struct {
		name           string
		driverName     string
		endpoint       string
		expectedDriver string
		expectedPort   int
	}{
		{
			name:           "Postgres with port",
			driverName:     "postgres",
			endpoint:       "host.example.com:5432",
			expectedDriver: "pgx",
			expectedPort:   5432,
		},
		{
			name:           "Postgres without port",
			driverName:     "postgres",
			endpoint:       "host.example.com",
			expectedDriver: "pgx",
			expectedPort:   5432,
		},
		{
			name:           "MySQL with port",
			driverName:     "mysql",
			endpoint:       "host.example.com:3306",
			expectedDriver: "mysql",
			expectedPort:   3306,
		},
		{
			name:           "MySQL without port",
			driverName:     "mysql",
			endpoint:       "host.example.com",
			expectedDriver: "mysql",
			expectedPort:   3306,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, port := determineDriverAndPort(tt.driverName, tt.endpoint)
			assert.Equal(t, tt.expectedDriver, driver)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

// TestCreateDBWithCredentialRefresh_ValidationErrors tests validation errors
func TestCreateDBWithCredentialRefresh_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	mockLogger := &MockLogger{}

	t.Run("IAM auth not enabled", func(t *testing.T) {
		config := ConnectionConfig{
			Driver: "postgres",
			DSN:    "postgres://user:password@host:5432/mydb",
		}

		_, err := createDBWithCredentialRefresh(ctx, config, mockLogger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS IAM auth not enabled")
	})

	t.Run("IAM auth enabled but nil config", func(t *testing.T) {
		config := ConnectionConfig{
			Driver:     "postgres",
			DSN:        "postgres://user:password@host:5432/mydb",
			AWSIAMAuth: nil,
		}

		_, err := createDBWithCredentialRefresh(ctx, config, mockLogger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS IAM auth not enabled")
	})

	t.Run("IAM auth config but disabled", func(t *testing.T) {
		config := ConnectionConfig{
			Driver: "postgres",
			DSN:    "postgres://user:password@host:5432/mydb",
			AWSIAMAuth: &AWSIAMAuthConfig{
				Enabled: false,
			},
		}

		_, err := createDBWithCredentialRefresh(ctx, config, mockLogger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AWS IAM auth not enabled")
	})
}

// TestDatabaseService_WithoutIAM ensures non-IAM connections still work
func TestDatabaseService_WithoutIAM(t *testing.T) {
	config := ConnectionConfig{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	service, err := NewDatabaseService(config, &MockLogger{})
	require.NoError(t, err)
	require.NotNil(t, service)

	err = service.Connect()
	require.NoError(t, err, "Should connect without IAM authentication")

	// Verify database connection is working
	db := service.DB()
	require.NotNil(t, db)

	ctx := context.Background()
	err = service.Ping(ctx)
	assert.NoError(t, err, "Should be able to ping database")

	// Test a simple query
	result, err := service.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Clean up
	err = service.Close()
	assert.NoError(t, err)
}

// TestDatabaseService_ConnectionPoolSettings tests that connection pool settings are applied
func TestDatabaseService_ConnectionPoolSettings(t *testing.T) {
	config := ConnectionConfig{
		Driver:                "sqlite",
		DSN:                   ":memory:",
		MaxOpenConnections:    20,
		MaxIdleConnections:    10,
		ConnectionMaxLifetime: 2 * time.Hour,
		ConnectionMaxIdleTime: 30 * time.Minute,
	}

	service, err := NewDatabaseService(config, &MockLogger{})
	require.NoError(t, err)

	err = service.Connect()
	require.NoError(t, err)

	// Verify connection pool settings
	db := service.DB()
	stats := db.Stats()
	assert.Equal(t, 20, stats.MaxOpenConnections)

	err = service.Close()
	assert.NoError(t, err)
}

// TestDatabaseService_IAMConfigValidation tests IAM configuration validation
func TestDatabaseService_IAMConfigValidation(t *testing.T) {
	t.Run("Missing region", func(t *testing.T) {
		// Skip actual AWS connection but test config structure
		config := ConnectionConfig{
			Driver: "postgres",
			DSN:    "postgres://user:password@host:5432/mydb",
			AWSIAMAuth: &AWSIAMAuthConfig{
				Enabled: true,
				Region:  "", // Missing region
				DBUser:  "testuser",
			},
		}

		service, err := NewDatabaseService(config, &MockLogger{})
		require.NoError(t, err) // Service creation should succeed
		require.NotNil(t, service)

		// Connection will fail due to missing region
		err = service.Connect()
		assert.Error(t, err) // Expected to fail without valid AWS config
	})

	t.Run("Missing DB user", func(t *testing.T) {
		config := ConnectionConfig{
			Driver: "postgres",
			DSN:    "postgres://user:password@host:5432/mydb",
			AWSIAMAuth: &AWSIAMAuthConfig{
				Enabled: true,
				Region:  "us-east-1",
				DBUser:  "", // Missing user
			},
		}

		service, err := NewDatabaseService(config, &MockLogger{})
		require.NoError(t, err)
		require.NotNil(t, service)

		// Connection will fail due to missing user
		err = service.Connect()
		assert.Error(t, err)
	})
}

// TestHelperFunctions_StillWork ensures helper functions from old implementation still work
func TestHelperFunctions_StillWork(t *testing.T) {
	t.Run("extractEndpointFromDSN", func(t *testing.T) {
		endpoint, err := extractEndpointFromDSN("postgres://user:password@host.example.com:5432/mydb")
		assert.NoError(t, err)
		assert.Equal(t, "host.example.com:5432", endpoint)
	})

	t.Run("replaceDSNPassword", func(t *testing.T) {
		newDSN, err := replaceDSNPassword("postgres://user:oldpass@host:5432/mydb", "newtoken")
		assert.NoError(t, err)
		assert.Contains(t, newDSN, "newtoken")
		assert.NotContains(t, newDSN, "oldpass")
	})

	t.Run("preprocessDSNForParsing", func(t *testing.T) {
		dsn, err := preprocessDSNForParsing("postgres://user:p@ss!word@host:5432/mydb")
		assert.NoError(t, err)
		assert.NotNil(t, dsn)
	})
}

// TestStripPasswordFromDSN tests removing passwords from DSN for backward compatibility
func TestStripPasswordFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "URL-style DSN with password",
			dsn:      "postgres://user:password@host:5432/mydb",
			expected: "postgres://user@host:5432/mydb",
		},
		{
			name:     "URL-style DSN with token",
			dsn:      "postgres://iamuser:some_token_placeholder@mydb.region.rds.amazonaws.com:5432/mydb",
			expected: "postgres://iamuser@mydb.region.rds.amazonaws.com:5432/mydb",
		},
		{
			name:     "URL-style DSN with $TOKEN placeholder",
			dsn:      "postgresql://myapp_user:$TOKEN@mydb-instance.cluster-abc123def456.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require",
			expected: "postgresql://myapp_user@mydb-instance.cluster-abc123def456.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require",
		},
		{
			name:     "URL-style DSN without password",
			dsn:      "postgres://user@host:5432/mydb",
			expected: "postgres://user@host:5432/mydb",
		},
		{
			name:     "URL-style DSN without credentials",
			dsn:      "postgres://host:5432/mydb",
			expected: "postgres://host:5432/mydb",
		},
		{
			name:     "Key-value DSN with password",
			dsn:      "host=localhost port=5432 user=testuser password=secret dbname=mydb",
			expected: "host=localhost port=5432 user=testuser dbname=mydb",
		},
		{
			name:     "Key-value DSN without password",
			dsn:      "host=localhost port=5432 user=testuser dbname=mydb",
			expected: "host=localhost port=5432 user=testuser dbname=mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripPasswordFromDSN(tt.dsn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestUsernameExtraction_WithTokenPlaceholder tests that username is correctly extracted
// even when DSN contains placeholder tokens like $TOKEN
func TestUsernameExtraction_WithTokenPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "DSN with $TOKEN placeholder",
			dsn:      "postgresql://myapp_user:$TOKEN@mydb-instance.cluster-abc123def456.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require",
			expected: "myapp_user",
		},
		{
			name:     "DSN with TOKEN placeholder (no $)",
			dsn:      "postgresql://db_user:TOKEN@myhost.rds.amazonaws.com:5432/mydb",
			expected: "db_user",
		},
		{
			name:     "DSN with actual token",
			dsn:      "postgresql://iam_user:some_actual_token_value@myhost.rds.amazonaws.com:5432/mydb",
			expected: "iam_user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractUsernameFromDSN(tt.dsn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractUsernameFromDSN tests extracting username from DSN
func TestExtractUsernameFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "URL-style DSN with password",
			dsn:      "postgres://myuser:password@host:5432/mydb",
			expected: "myuser",
		},
		{
			name:     "URL-style DSN without password",
			dsn:      "postgres://myuser@host:5432/mydb",
			expected: "myuser",
		},
		{
			name:     "URL-style DSN without credentials",
			dsn:      "postgres://host:5432/mydb",
			expected: "",
		},
		{
			name:     "Key-value DSN with user",
			dsn:      "host=localhost port=5432 user=testuser dbname=mydb",
			expected: "testuser",
		},
		{
			name:     "Key-value DSN without user",
			dsn:      "host=localhost port=5432 dbname=mydb",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractUsernameFromDSN(tt.dsn)
			assert.Equal(t, tt.expected, result)
		})
	}
}
