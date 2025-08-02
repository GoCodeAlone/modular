// Package httpserver provides an HTTP server module for the modular framework.
package httpserver

import (
	"errors"
	"fmt"
	"time"
)

// DefaultTimeoutSeconds is the default timeout value in seconds
const DefaultTimeoutSeconds = 15

// Static error definitions for better error handling
var (
	ErrInvalidPortNumber          = errors.New("invalid port number")
	ErrTLSAutoGenerationNoDomains = errors.New("TLS auto-generation is enabled but no domains specified")
	ErrTLSNoCertificateFile       = errors.New("TLS is enabled but no certificate file specified")
	ErrTLSNoKeyFile               = errors.New("TLS is enabled but no key file specified")
	ErrRouterNotHTTPHandler       = errors.New("service does not implement http.Handler")
	ErrServerStartTimeout         = errors.New("context cancelled while waiting for server to start")
)

// HTTPServerConfig defines the configuration for the HTTP server module.
type HTTPServerConfig struct {
	// Host is the hostname or IP address to bind to.
	Host string `yaml:"host" json:"host" env:"HOST"`

	// Port is the port number to listen on.
	Port int `yaml:"port" json:"port" env:"PORT"`

	// ReadTimeout is the maximum duration for reading the entire request,
	// including the body, in seconds.
	ReadTimeout int `yaml:"read_timeout" json:"read_timeout" env:"READ_TIMEOUT"`

	// WriteTimeout is the maximum duration before timing out writes of the response,
	// in seconds.
	WriteTimeout int `yaml:"write_timeout" json:"write_timeout" env:"WRITE_TIMEOUT"`

	// IdleTimeout is the maximum amount of time to wait for the next request,
	// in seconds.
	IdleTimeout int `yaml:"idle_timeout" json:"idle_timeout" env:"IDLE_TIMEOUT"`

	// ShutdownTimeout is the maximum amount of time to wait during graceful
	// shutdown, in seconds.
	ShutdownTimeout int `yaml:"shutdown_timeout" json:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT"`

	// TLS configuration if HTTPS is enabled
	TLS *TLSConfig `yaml:"tls" json:"tls"`
}

// TLSConfig holds the TLS configuration for HTTPS support
type TLSConfig struct {
	// Enabled indicates if HTTPS should be used instead of HTTP
	Enabled bool `yaml:"enabled" json:"enabled" env:"TLS_ENABLED"`

	// CertFile is the path to the certificate file
	CertFile string `yaml:"cert_file" json:"cert_file" env:"TLS_CERT_FILE"`

	// KeyFile is the path to the private key file
	KeyFile string `yaml:"key_file" json:"key_file" env:"TLS_KEY_FILE"`

	// UseService indicates whether to use a certificate service instead of files
	// When true, the module will look for a CertificateService in its dependencies
	UseService bool `yaml:"use_service" json:"use_service" env:"TLS_USE_SERVICE"`

	// AutoGenerate indicates whether to automatically generate self-signed certificates
	// if no certificate service is provided and file paths are not specified
	AutoGenerate bool `yaml:"auto_generate" json:"auto_generate" env:"TLS_AUTO_GENERATE"`

	// Domains is a list of domain names to generate certificates for (when AutoGenerate is true)
	Domains []string `yaml:"domains" json:"domains" env:"TLS_DOMAINS"`
}

// Validate checks if the configuration is valid and sets default values
// where appropriate.
func (c *HTTPServerConfig) Validate() error {
	// Set default host if not specified
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}

	// Set default port if not specified
	if c.Port == 0 {
		c.Port = 8080
	}

	// Check if port is within valid range
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("%w: %d", ErrInvalidPortNumber, c.Port)
	}

	// Set default timeouts if not specified
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 15 // 15 seconds
	}

	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 15 // 15 seconds
	}

	if c.IdleTimeout <= 0 {
		c.IdleTimeout = 60 // 60 seconds
	}

	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 30 // 30 seconds
	}

	// Validate TLS configuration if enabled
	if c.TLS != nil && c.TLS.Enabled {
		// If using service, we don't need cert/key files
		if c.TLS.UseService {
			// UseService takes precedence over file-based configuration
			return nil
		}

		// If AutoGenerate is true, we don't need cert/key files
		if c.TLS.AutoGenerate {
			// Make sure we have at least one domain for auto-generated certs
			if len(c.TLS.Domains) == 0 {
				return ErrTLSAutoGenerationNoDomains
			}
			return nil
		}

		// Otherwise, we need cert/key files
		if c.TLS.CertFile == "" {
			return ErrTLSNoCertificateFile
		}
		if c.TLS.KeyFile == "" {
			return ErrTLSNoKeyFile
		}
	}

	return nil
}

// GetTimeout converts a timeout value from seconds to time.Duration.
// If seconds is 0, it returns the default timeout.
func (c *HTTPServerConfig) GetTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = DefaultTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
