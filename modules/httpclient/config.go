// Package httpclient provides a configurable HTTP client module for the modular framework.
package httpclient

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrLogFilePathRequired is returned when log_to_file is enabled but log_file_path is not specified
	ErrLogFilePathRequired = errors.New("log_file_path must be specified when log_to_file is enabled")
)

// Config defines the configuration for the HTTP client module.
// This structure contains all the settings needed to configure HTTP client
// behavior, connection pooling, timeouts, and logging.
//
// Configuration can be provided through JSON, YAML, or environment variables.
// The struct tags define the mapping for each configuration source.
//
// Example YAML configuration:
//
//	max_idle_conns: 200
//	max_idle_conns_per_host: 20
//	idle_conn_timeout: 120
//	request_timeout: 60
//	tls_timeout: 15
//	disable_compression: false
//	disable_keep_alives: false
//	verbose: true
//	verbose_options:
//	  log_headers: true
//	  log_body: true
//	  max_body_log_size: 1024
//	  log_to_file: true
//	  log_file_path: "/var/log/httpclient"
//
// Example environment variables:
//
//	HTTPCLIENT_MAX_IDLE_CONNS=200
//	HTTPCLIENT_REQUEST_TIMEOUT=60
//	HTTPCLIENT_VERBOSE=true
type Config struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	// This setting affects the total connection pool size and memory usage.
	// Higher values allow more concurrent connections but use more memory.
	// Default: 100
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns" env:"MAX_IDLE_CONNS"`

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	// This prevents a single host from monopolizing the connection pool.
	// Should be tuned based on expected traffic patterns to specific hosts.
	// Default: 10
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host" env:"MAX_IDLE_CONNS_PER_HOST"`

	// IdleConnTimeout is the maximum amount of time an idle connection will remain idle
	// before closing itself. This helps prevent stale connections and
	// reduces server-side resource usage.
	// Default: 90 seconds
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout" json:"idle_conn_timeout" env:"IDLE_CONN_TIMEOUT"`

	// RequestTimeout is the maximum time for a request to complete.
	// This includes connection time, any redirects, and reading the response body.
	// Use WithTimeout() method for per-request timeout overrides.
	// Default: 30 seconds
	RequestTimeout time.Duration `yaml:"request_timeout" json:"request_timeout" env:"REQUEST_TIMEOUT"`

	// TLSTimeout is the maximum time waiting for TLS handshake.
	// This only affects HTTPS connections and should be set based on expected
	// network latency and certificate chain complexity.
	// Default: 10 seconds
	TLSTimeout time.Duration `yaml:"tls_timeout" json:"tls_timeout" env:"TLS_TIMEOUT"`

	// DisableCompression disables decompressing response bodies.
	// When false (default), the client automatically handles gzip compression.
	// Set to true if you need to handle compression manually or want raw responses.
	// Default: false (compression enabled)
	DisableCompression bool `yaml:"disable_compression" json:"disable_compression" env:"DISABLE_COMPRESSION"`

	// DisableKeepAlives disables HTTP keep-alive and will only use connections for a single request.
	// This can be useful for debugging or when connecting to servers that don't handle
	// keep-alives properly, but significantly impacts performance.
	// Default: false (keep-alives enabled)
	DisableKeepAlives bool `yaml:"disable_keep_alives" json:"disable_keep_alives" env:"DISABLE_KEEP_ALIVES"`

	// Verbose enables detailed logging of HTTP requests and responses.
	// When enabled, logs include request/response headers, bodies, timing information,
	// and error details. Very useful for debugging but can impact performance.
	// Default: false
	Verbose bool `yaml:"verbose" json:"verbose" env:"VERBOSE"`

	// VerboseOptions configures the behavior when Verbose is enabled.
	// This allows fine-grained control over what gets logged and where.
	VerboseOptions *VerboseOptions `yaml:"verbose_options" json:"verbose_options" env:"VERBOSE_OPTIONS"`
}

// VerboseOptions configures the behavior of verbose logging.
// These options provide fine-grained control over HTTP request/response logging
// to balance debugging needs with performance and security considerations.
type VerboseOptions struct {
	// LogHeaders enables logging of request and response headers.
	// This includes all HTTP headers sent and received, which can contain
	// sensitive information like authorization tokens.
	// Default: false
	LogHeaders bool `yaml:"log_headers" json:"log_headers" env:"LOG_HEADERS"`

	// LogBody enables logging of request and response bodies.
	// This can generate large amounts of log data and may contain sensitive
	// information. Consider using MaxBodyLogSize to limit logged content.
	// Default: false
	LogBody bool `yaml:"log_body" json:"log_body" env:"LOG_BODY"`

	// MaxBodyLogSize limits the size of logged request and response bodies.
	// Bodies larger than this size will be truncated in logs. Set to 0 for no limit.
	// Helps prevent log spam from large file uploads or downloads.
	// Default: 0 (no limit)
	MaxBodyLogSize int `yaml:"max_body_log_size" json:"max_body_log_size" env:"MAX_BODY_LOG_SIZE"`

	// LogToFile enables logging to files instead of just the application logger.
	// When enabled, HTTP logs are written to separate files for easier analysis.
	// Requires LogFilePath to be set.
	// Default: false
	LogToFile bool `yaml:"log_to_file" json:"log_to_file" env:"LOG_TO_FILE"`

	// LogFilePath is the directory where log files will be written.
	// Log files are organized by date and include request/response details.
	// The directory must be writable by the application.
	// Default: "" (current directory)
	LogFilePath string `yaml:"log_file_path" json:"log_file_path" env:"LOG_FILE_PATH"`
}

// Validate checks the configuration values and sets sensible defaults.
func (c *Config) Validate() error {
	// Set defaults for connection pooling
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 100
	}

	if c.MaxIdleConnsPerHost <= 0 {
		c.MaxIdleConnsPerHost = 10
	}

	// Set timeout defaults if zero values (programmatic defaults work reliably)
	if c.IdleConnTimeout == 0 {
		c.IdleConnTimeout = 90 * time.Second
	}

	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}

	if c.TLSTimeout == 0 {
		c.TLSTimeout = 10 * time.Second
	}

	// Initialize verbose options if needed
	if c.Verbose && c.VerboseOptions == nil {
		c.VerboseOptions = &VerboseOptions{
			LogHeaders:     true,
			LogBody:        true,
			MaxBodyLogSize: 10000, // 10KB
			LogToFile:      false,
		}
	}

	// Validate verbose log file path if logging to file is enabled
	if c.Verbose && c.VerboseOptions != nil && c.VerboseOptions.LogToFile && c.VerboseOptions.LogFilePath == "" {
		return fmt.Errorf("config validation error: %w", ErrLogFilePathRequired)
	}

	return nil
}
