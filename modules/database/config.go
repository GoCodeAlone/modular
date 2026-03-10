package database

import (
	"time"
)

// Config represents database module configuration
type Config struct {
	// Connections contains all defined database connections
	Connections map[string]*ConnectionConfig `json:"connections" yaml:"connections"`

	// Default specifies the name of the default connection
	Default string `json:"default" yaml:"default" env:"DEFAULT_DB_CONNECTION"`
}

// Validate implements ConfigValidator interface
func (c *Config) Validate() error {
	// Add any validation logic here if needed
	return nil
}

// GetInstanceConfigs returns the connections map for instance-aware configuration
func (c *Config) GetInstanceConfigs() map[string]interface{} {
	instances := make(map[string]interface{})
	for name, connection := range c.Connections {
		// Return pointers to the original connection configs so they can be modified
		instances[name] = connection
	}
	return instances
}

// ConnectionConfig represents configuration for a single database connection
type ConnectionConfig struct {
	// Driver specifies the database driver to use (e.g., "postgres", "mysql")
	Driver string `json:"driver" yaml:"driver" env:"DRIVER"`

	// DSN is the database connection string
	DSN string `json:"dsn" yaml:"dsn" env:"DSN"`

	// MaxOpenConnections sets the maximum number of open connections to the database
	MaxOpenConnections int `json:"max_open_connections" yaml:"max_open_connections" env:"MAX_OPEN_CONNECTIONS"`

	// MaxIdleConnections sets the maximum number of idle connections in the pool
	MaxIdleConnections int `json:"max_idle_connections" yaml:"max_idle_connections" env:"MAX_IDLE_CONNECTIONS"`

	// ConnectionMaxLifetime sets the maximum amount of time a connection may be reused
	ConnectionMaxLifetime time.Duration `json:"connection_max_lifetime" yaml:"connection_max_lifetime" env:"CONNECTION_MAX_LIFETIME"`

	// ConnectionMaxIdleTime sets the maximum amount of time a connection may be idle
	ConnectionMaxIdleTime time.Duration `json:"connection_max_idle_time" yaml:"connection_max_idle_time" env:"CONNECTION_MAX_IDLE_TIME"`

	// AWSIAMAuth contains AWS IAM authentication configuration
	AWSIAMAuth *AWSIAMAuthConfig `json:"aws_iam_auth,omitempty" yaml:"aws_iam_auth,omitempty"`
}

// AWSIAMAuthConfig represents AWS IAM authentication configuration.
//
// When IAM authentication is enabled, the database module uses AWS RDS IAM authentication
// to automatically generate and refresh database authentication tokens. This provides several benefits:
//   - No need to manage database passwords
//   - Tokens are automatically rotated (15-minute lifetime)
//   - Uses AWS IAM for access control
//   - Audit trail through AWS CloudTrail
//
// DSN Password Handling:
// When IAM authentication is enabled, any password in the DSN is ignored and stripped.
// You can include a placeholder like $TOKEN in the DSN for clarity, but it will be removed:
//
//	Example DSN:
//	  postgresql://myapp_user:$TOKEN@mydb.us-east-1.rds.amazonaws.com:5432/mydb?sslmode=require
//
//	The module will:
//	  1. Strip the "$TOKEN" placeholder from the DSN
//	  2. Extract the username "myapp_user"
//	  3. Use AWS credentials to generate an IAM auth token
//	  4. Automatically refresh the token before it expires
//
// Configuration Options:
type AWSIAMAuthConfig struct {
	// Enabled indicates whether AWS IAM authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled" env:"AWS_IAM_AUTH_ENABLED"`

	// Region specifies the AWS region for the RDS instance (e.g., "us-east-1")
	// This is required for IAM token generation
	Region string `json:"region" yaml:"region" env:"AWS_IAM_AUTH_REGION"`

	// DBUser specifies the database username for IAM authentication.
	// If not specified, the username will be extracted from the DSN.
	// This field takes precedence over the username in the DSN if both are provided.
	DBUser string `json:"db_user" yaml:"db_user" env:"AWS_IAM_AUTH_DB_USER"`

	// ConnectionTimeout specifies the timeout for database connection tests
	// Default is 5 seconds
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout" env:"AWS_IAM_AUTH_CONNECTION_TIMEOUT" default:"5s"`
}
