package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/GoCodeAlone/modular"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/davepgreene/go-db-credential-refresh/driver"
	"github.com/davepgreene/go-db-credential-refresh/store/awsrds"
)

var (
	ErrAWSIAMAuthNotEnabled = errors.New("AWS IAM auth not enabled")
	ErrInvalidDSNFormat     = errors.New("invalid DSN format")
	ErrMissingIAMRegion     = errors.New("AWS IAM auth region is required but not configured")
	ErrMissingIAMUsername   = errors.New("database username is required for IAM auth but not found in DSN or config")
)

// createDBWithCredentialRefresh creates a database connection using go-db-credential-refresh.
// This automatically handles token refresh and connection recreation on auth errors.
//
// AWS IAM Authentication Flow:
//  1. Any password/token in the DSN is stripped (including placeholders like $TOKEN)
//  2. Username is extracted from the DSN (e.g., "chimera_app" from postgresql://chimera_app:$TOKEN@host/db)
//  3. AWS credentials are loaded from the environment/instance profile/etc.
//  4. RDS IAM auth token is automatically generated using AWS credentials
//  5. Token is automatically refreshed before expiration
//  6. Connection is automatically recreated on authentication errors
//
// This means you can pass a DSN with a placeholder token like:
//
//	postgresql://chimera_app:$TOKEN@shared-chimera-dev-backend.cluster-xyz.us-east-1.rds.amazonaws.com:5432/chimera_backend?sslmode=require
//
// And the module will:
//   - Ignore the "$TOKEN" placeholder
//   - Use "chimera_app" as the database username for IAM authentication
//   - Automatically generate and refresh the actual IAM token
//
// Configuration Requirements:
//   - AWSIAMAuth.Enabled must be true
//   - AWSIAMAuth.Region must be set to the RDS instance region
//   - AWSIAMAuth.DBUser can optionally override the username from the DSN
//   - AWS credentials must be available (environment variables, instance profile, etc.)
func createDBWithCredentialRefresh(ctx context.Context, connConfig ConnectionConfig, logger modular.Logger) (*sql.DB, error) {
	if connConfig.AWSIAMAuth == nil || !connConfig.AWSIAMAuth.Enabled {
		return nil, ErrAWSIAMAuthNotEnabled
	}

	logger.Info("Starting AWS IAM authentication setup",
		"region", connConfig.AWSIAMAuth.Region,
		"driver", connConfig.Driver)

	// Validate configuration
	if connConfig.AWSIAMAuth.Region == "" {
		logger.Error("AWS IAM authentication requires a region", "config_region", connConfig.AWSIAMAuth.Region)
		return nil, ErrMissingIAMRegion
	}

	// Load AWS configuration
	logger.Debug("Loading AWS configuration", "region", connConfig.AWSIAMAuth.Region)
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(connConfig.AWSIAMAuth.Region))
	if err != nil {
		logger.Error("Failed to load AWS configuration",
			"error", err.Error(),
			"region", connConfig.AWSIAMAuth.Region,
			"possible_causes", "Missing AWS credentials, invalid region, or network issues")
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w (check AWS credentials are available)", connConfig.AWSIAMAuth.Region, err)
	}
	logger.Debug("AWS configuration loaded successfully")

	// Strip any existing password/token from DSN for backward compatibility
	// This allows applications that previously passed DSN with token placeholders to continue working
	logger.Debug("Processing DSN for IAM authentication", "original_dsn_length", len(connConfig.DSN))
	cleanDSN := stripPasswordFromDSN(connConfig.DSN)
	logger.Debug("Password stripped from DSN", "cleaned_dsn_length", len(cleanDSN))

	// Extract endpoint from DSN
	endpoint, err := extractEndpointFromDSN(cleanDSN)
	if err != nil {
		logger.Error("Failed to extract RDS endpoint from DSN",
			"error", err.Error(),
			"dsn_format", "Expected format: postgresql://user@host:port/db or postgres://user@host/db",
			"troubleshooting", "Verify DSN format is correct")
		return nil, fmt.Errorf("failed to extract endpoint from DSN: %w (check DSN format)", err)
	}
	logger.Info("Extracted RDS endpoint", "endpoint", endpoint)

	// Extract database name and options from DSN
	dbName, opts, err := extractDatabaseAndOptions(cleanDSN)
	if err != nil {
		logger.Error("Failed to extract database name from DSN",
			"error", err.Error(),
			"dsn_sample", "postgresql://user@host:port/database_name")
		return nil, fmt.Errorf("failed to extract database name and options: %w", err)
	}
	logger.Debug("Extracted database configuration", "database", dbName, "options_count", len(opts))

	// Extract username from DSN if present, otherwise use config
	username := extractUsernameFromDSN(cleanDSN)
	if username == "" {
		username = connConfig.AWSIAMAuth.DBUser
		logger.Debug("Using username from config", "username", username)
	} else {
		logger.Debug("Extracted username from DSN", "username", username)
	}

	// Validate username is present
	if username == "" {
		logger.Error("Database username not found",
			"dsn_has_username", false,
			"config_has_db_user", connConfig.AWSIAMAuth.DBUser != "",
			"troubleshooting", "Either include username in DSN or set aws_iam_auth.db_user in config")
		return nil, ErrMissingIAMUsername
	}
	logger.Info("IAM authentication will use database user", "username", username)

	// Determine driver name and port
	driverName, port := determineDriverAndPort(connConfig.Driver, endpoint)
	logger.Debug("Determined database driver configuration", "driver", driverName, "port", port)

	// Create AWS RDS store for credential management
	logger.Info("Creating AWS RDS credential store",
		"endpoint", endpoint,
		"region", connConfig.AWSIAMAuth.Region,
		"username", username)
	awsStore, err := awsrds.NewStore(&awsrds.Config{
		Credentials: awsConfig.Credentials,
		Endpoint:    endpoint,
		Region:      connConfig.AWSIAMAuth.Region,
		User:        username,
	})
	if err != nil {
		logger.Error("Failed to create AWS RDS credential store",
			"error", err.Error(),
			"endpoint", endpoint,
			"region", connConfig.AWSIAMAuth.Region,
			"username", username,
			"possible_causes", "Invalid AWS credentials, network issues, or incorrect endpoint")
		return nil, fmt.Errorf("failed to create AWS RDS store for endpoint %s: %w", endpoint, err)
	}
	logger.Debug("AWS RDS credential store created successfully")

	// CRITICAL FIX: Wrap the AWS store with TTL-based caching
	// The awsrds.Store caches credentials indefinitely, which causes PAM failures
	// after 15 minutes when tokens expire. Our TTL wrapper ensures credentials
	// are refreshed before expiration (14 minutes).
	store := NewTTLStore(awsStore)
	logger.Info("Wrapped AWS RDS store with TTL-based token refresh",
		"token_lifetime", EffectiveTokenLifetime,
		"refresh_before_expiration", TokenRefreshBuffer)

	// Extract hostname from endpoint (remove port)
	hostname := endpoint
	if colonIdx := strings.LastIndex(endpoint, ":"); colonIdx != -1 {
		hostname = endpoint[:colonIdx]
	}
	logger.Debug("Extracted hostname from endpoint", "hostname", hostname, "original_endpoint", endpoint)

	// Create connector configuration
	cfg := &driver.Config{
		Host:    hostname,
		Port:    port,
		DB:      dbName,
		Opts:    opts,
		Retries: 1, // Retry once on auth failure
	}
	logger.Debug("Created database connector configuration",
		"host", hostname,
		"port", port,
		"database", dbName,
		"retries", cfg.Retries)

	// Create connector with credential refresh
	logger.Info("Creating database connector with automatic credential refresh")
	connector, err := driver.NewConnector(store, driverName, cfg)
	if err != nil {
		logger.Error("Failed to create database connector",
			"error", err.Error(),
			"driver", driverName,
			"host", hostname,
			"port", port,
			"troubleshooting", "Check driver compatibility and configuration")
		return nil, fmt.Errorf("failed to create connector for %s: %w", driverName, err)
	}
	logger.Debug("Database connector created successfully")

	// Open database using the connector
	logger.Info("Opening database connection with IAM authentication")
	db := sql.OpenDB(connector)

	// Configure connection pool
	logger.Debug("Configuring connection pool",
		"max_open", connConfig.MaxOpenConnections,
		"max_idle", connConfig.MaxIdleConnections)
	configureConnectionPool(db, connConfig)

	logger.Info("Database connection with AWS IAM authentication configured successfully",
		"endpoint", endpoint,
		"username", username,
		"database", dbName)

	return db, nil
}

// configureConnectionPool applies connection pool settings to a database connection
func configureConnectionPool(db *sql.DB, config ConnectionConfig) {
	if config.MaxOpenConnections > 0 {
		db.SetMaxOpenConns(config.MaxOpenConnections)
	}
	if config.MaxIdleConnections > 0 {
		db.SetMaxIdleConns(config.MaxIdleConnections)
	}
	if config.ConnectionMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnectionMaxLifetime)
	}
	if config.ConnectionMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(config.ConnectionMaxIdleTime)
	}
}

// stripPasswordFromDSN removes any password from a DSN for backward compatibility.
// This allows applications that previously passed DSN with token placeholders to continue working.
//
// When AWS IAM authentication is enabled, the password portion of the DSN is ignored and stripped.
// This is intentional because:
//   - AWS IAM tokens are automatically generated by the go-db-credential-refresh library
//   - Any placeholder value (e.g., $TOKEN, TOKEN, or any other string) in the password field will be removed
//   - The actual IAM token is obtained using AWS credentials and the username extracted from the DSN
//
// Example DSNs that will have their passwords stripped:
//   - postgresql://chimera_app:$TOKEN@host.rds.amazonaws.com:5432/mydb?sslmode=require
//     becomes: postgresql://chimera_app@host.rds.amazonaws.com:5432/mydb?sslmode=require
//   - postgres://user:some_placeholder@host:5432/mydb
//     becomes: postgres://user@host:5432/mydb
//
// The username (e.g., "chimera_app") is preserved and used for IAM authentication.
func stripPasswordFromDSN(dsn string) string {
	if strings.Contains(dsn, "://") {
		// URL-style DSN (e.g., postgres://user:password@host:port/database)
		// Find user info section
		schemeEnd := strings.Index(dsn, "://")
		if schemeEnd == -1 {
			return dsn
		}

		atIdx := strings.Index(dsn[schemeEnd+3:], "@")
		if atIdx == -1 {
			return dsn // No credentials
		}

		// Extract scheme and credentials section
		scheme := dsn[:schemeEnd+3]
		credentials := dsn[schemeEnd+3 : schemeEnd+3+atIdx]
		remainder := dsn[schemeEnd+3+atIdx:]

		// Check if there's a password (contains colon)
		colonIdx := strings.Index(credentials, ":")
		if colonIdx == -1 {
			return dsn // No password
		}

		// Rebuild DSN without password
		username := credentials[:colonIdx]
		return scheme + username + remainder
	}

	// Key-value style DSN (e.g., host=localhost port=5432 password=token dbname=mydb)
	parts := strings.Fields(dsn)
	var result []string
	for _, part := range parts {
		if !strings.HasPrefix(part, "password=") {
			result = append(result, part)
		}
	}
	return strings.Join(result, " ")
}

// extractUsernameFromDSN extracts the username from a DSN
func extractUsernameFromDSN(dsn string) string {
	if strings.Contains(dsn, "://") {
		// URL-style DSN
		schemeEnd := strings.Index(dsn, "://")
		if schemeEnd == -1 {
			return ""
		}

		atIdx := strings.Index(dsn[schemeEnd+3:], "@")
		if atIdx == -1 {
			return "" // No credentials
		}

		credentials := dsn[schemeEnd+3 : schemeEnd+3+atIdx]

		// Check if there's a colon (username:password format)
		colonIdx := strings.Index(credentials, ":")
		if colonIdx != -1 {
			return credentials[:colonIdx]
		}

		// No colon means just username
		return credentials
	}

	// Key-value style DSN
	parts := strings.Fields(dsn)
	for _, part := range parts {
		if strings.HasPrefix(part, "user=") {
			return strings.TrimPrefix(part, "user=")
		}
	}
	return ""
}

// extractDatabaseAndOptions extracts the database name and connection options from a DSN
func extractDatabaseAndOptions(dsn string) (string, map[string]string, error) {
	opts := make(map[string]string)

	if strings.Contains(dsn, "://") {
		// URL-style DSN (e.g., postgres://user:password@host:port/database?option=value)
		// Parse URL to get path (database) and query params (options)
		parts := strings.Split(dsn, "://")
		if len(parts) != 2 {
			return "", nil, ErrInvalidDSNFormat
		}

		remainder := parts[1]

		// Find database name (after last / before ?)
		dbStart := strings.LastIndex(remainder, "/")
		if dbStart == -1 {
			return "", opts, nil // No database specified
		}

		dbPart := remainder[dbStart+1:]
		dbName := dbPart

		// Extract query parameters if present
		if qIdx := strings.Index(dbPart, "?"); qIdx != -1 {
			dbName = dbPart[:qIdx]
			queryString := dbPart[qIdx+1:]

			// Parse query parameters
			for _, pair := range strings.Split(queryString, "&") {
				if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 {
					opts[kv[0]] = kv[1]
				}
			}
		}

		return dbName, opts, nil
	}

	// Key-value style DSN (e.g., host=localhost port=5432 dbname=mydb sslmode=disable)
	parts := strings.Fields(dsn)
	dbName := ""

	for _, part := range parts {
		if strings.HasPrefix(part, "dbname=") {
			dbName = strings.TrimPrefix(part, "dbname=")
		} else if !strings.HasPrefix(part, "host=") &&
			!strings.HasPrefix(part, "port=") &&
			!strings.HasPrefix(part, "user=") &&
			!strings.HasPrefix(part, "password=") {
			// Extract as option
			if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
				opts[kv[0]] = kv[1]
			}
		}
	}

	return dbName, opts, nil
}

// determineDriverAndPort determines the correct driver name for go-db-credential-refresh
// and extracts the port from the endpoint
func determineDriverAndPort(driverName string, endpoint string) (string, int) {
	// Set default port based on driver
	defaultPort := 5432
	driver := "pgx"
	if driverName == "mysql" {
		defaultPort = 3306
		driver = "mysql"
	}
	port := defaultPort

	// Override with explicit port if present
	if colonIdx := strings.LastIndex(endpoint, ":"); colonIdx != -1 {
		if n, err := fmt.Sscanf(endpoint[colonIdx+1:], "%d", &port); err != nil || n != 1 {
			port = defaultPort // Restore default if parsing fails
		}
	}

	return driver, port
}
