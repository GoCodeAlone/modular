// Package httpclient provides a configurable HTTP client module for the modular framework.
package httpclient

import (
	"fmt"
	"time"
)

// Config defines the configuration for the HTTP client module.
type Config struct {
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`

	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host" json:"max_idle_conns_per_host"`

	// IdleConnTimeout is the maximum amount of time an idle connection will remain idle before
	// closing itself, in seconds.
	IdleConnTimeout int `yaml:"idle_conn_timeout" json:"idle_conn_timeout"`

	// RequestTimeout is the maximum time for a request to complete, in seconds.
	RequestTimeout int `yaml:"request_timeout" json:"request_timeout"`

	// TLSTimeout is the maximum time waiting for TLS handshake, in seconds.
	TLSTimeout int `yaml:"tls_timeout" json:"tls_timeout"`

	// DisableCompression disables decompressing response bodies.
	DisableCompression bool `yaml:"disable_compression" json:"disable_compression"`

	// DisableKeepAlives disables HTTP keep-alive and will only use connections for a single request.
	DisableKeepAlives bool `yaml:"disable_keep_alives" json:"disable_keep_alives"`

	// Verbose enables detailed logging of HTTP requests and responses.
	Verbose bool `yaml:"verbose" json:"verbose"`

	// VerboseOptions configures the behavior when Verbose is enabled.
	VerboseOptions *VerboseOptions `yaml:"verbose_options" json:"verbose_options"`
}

// VerboseOptions configures the behavior of verbose logging.
type VerboseOptions struct {
	// LogHeaders enables logging of request and response headers.
	LogHeaders bool `yaml:"log_headers" json:"log_headers"`

	// LogBody enables logging of request and response bodies.
	LogBody bool `yaml:"log_body" json:"log_body"`

	// MaxBodyLogSize limits the size of logged request and response bodies.
	MaxBodyLogSize int `yaml:"max_body_log_size" json:"max_body_log_size"`

	// LogToFile enables logging to files instead of just the logger.
	LogToFile bool `yaml:"log_to_file" json:"log_to_file"`

	// LogFilePath is the directory where log files will be written.
	LogFilePath string `yaml:"log_file_path" json:"log_file_path"`
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

	// Set default timeout values
	if c.IdleConnTimeout <= 0 {
		c.IdleConnTimeout = 90 // 90 seconds
	}

	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 30 // 30 seconds
	}

	if c.TLSTimeout <= 0 {
		c.TLSTimeout = 10 // 10 seconds
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
		return fmt.Errorf("log_file_path must be specified when log_to_file is enabled")
	}

	return nil
}

// GetTimeout converts a timeout value from seconds to time.Duration.
func (c *Config) GetTimeout(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
