package database

// Config represents database module configuration
type Config struct {
	// Connections contains all defined database connections
	Connections map[string]ConnectionConfig `json:"connections" yaml:"connections"`

	// Default specifies the name of the default connection
	Default string `json:"default" yaml:"default"`
}

// ConnectionConfig represents configuration for a single database connection
type ConnectionConfig struct {
	// Driver specifies the database driver to use (e.g., "postgres", "mysql")
	Driver string `json:"driver" yaml:"driver"`

	// DSN is the database connection string
	DSN string `json:"dsn" yaml:"dsn"`

	// MaxOpenConnections sets the maximum number of open connections to the database
	MaxOpenConnections int `json:"max_open_connections" yaml:"max_open_connections"`

	// MaxIdleConnections sets the maximum number of idle connections in the pool
	MaxIdleConnections int `json:"max_idle_connections" yaml:"max_idle_connections"`

	// ConnectionMaxLifetime sets the maximum amount of time a connection may be reused (in seconds)
	ConnectionMaxLifetime int `json:"connection_max_lifetime" yaml:"connection_max_lifetime"`

	// ConnectionMaxIdleTime sets the maximum amount of time a connection may be idle (in seconds)
	ConnectionMaxIdleTime int `json:"connection_max_idle_time" yaml:"connection_max_idle_time"`

	// AWSIAMAuth contains AWS IAM authentication configuration
	AWSIAMAuth *AWSIAMAuthConfig `json:"aws_iam_auth,omitempty" yaml:"aws_iam_auth,omitempty"`
}

// AWSIAMAuthConfig represents AWS IAM authentication configuration
type AWSIAMAuthConfig struct {
	// Enabled indicates whether AWS IAM authentication is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Region specifies the AWS region for the RDS instance
	Region string `json:"region" yaml:"region"`

	// DBUser specifies the database username for IAM authentication
	DBUser string `json:"db_user" yaml:"db_user"`

	// TokenRefreshInterval specifies how often to refresh the IAM token (in seconds)
	// Default is 10 minutes (600 seconds), tokens expire after 15 minutes
	TokenRefreshInterval int `json:"token_refresh_interval" yaml:"token_refresh_interval"`
}
