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

// AWSIAMAuthConfig represents AWS IAM authentication configuration
type AWSIAMAuthConfig struct {
	// Enabled indicates whether AWS IAM authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled" env:"AWS_IAM_AUTH_ENABLED"`

	// Region specifies the AWS region for the RDS instance
	Region string `json:"region" yaml:"region" env:"AWS_IAM_AUTH_REGION"`

	// DBUser specifies the database username for IAM authentication
	DBUser string `json:"db_user" yaml:"db_user" env:"AWS_IAM_AUTH_DB_USER"`

	// TokenRefreshInterval specifies how often to refresh the IAM token (in seconds)
	// Default is 10 minutes (600 seconds), tokens expire after 15 minutes
	TokenRefreshInterval int `json:"token_refresh_interval" yaml:"token_refresh_interval" env:"AWS_IAM_AUTH_TOKEN_REFRESH" default:"600"`
}
