package chimux

import (
	"time"
)

// ChiMuxConfig holds the configuration for the chimux module.
// This structure contains all the settings needed to configure CORS,
// request handling, and routing behavior for the Chi router.
//
// Configuration can be provided through JSON, YAML, or environment variables.
// The struct tags define the mapping for each configuration source and
// default values.
//
// Example YAML configuration:
//
//	allowed_origins:
//	  - "https://example.com"
//	  - "https://app.example.com"
//	allowed_methods:
//	  - "GET"
//	  - "POST"
//	  - "PUT"
//	  - "DELETE"
//	allowed_headers:
//	  - "Origin"
//	  - "Accept"
//	  - "Content-Type"
//	  - "Authorization"
//	allow_credentials: true
//	max_age: 3600
//	timeout: 30000
//	basepath: "/api/v1"
//
// Example environment variables:
//
//	CHIMUX_ALLOWED_ORIGINS=https://example.com,https://app.example.com
//	CHIMUX_ALLOW_CREDENTIALS=true
//	CHIMUX_BASE_PATH=/api/v1
type ChiMuxConfig struct {
	// AllowedOrigins specifies the list of allowed origins for CORS requests.
	// Use ["*"] to allow all origins, or specify exact origins for security.
	// Multiple origins can be specified for multi-domain applications.
	// Default: ["*"]
	AllowedOrigins []string `yaml:"allowed_origins" default:"[\"*\"]" desc:"List of allowed origins for CORS requests." env:"ALLOWED_ORIGINS"`

	// AllowedMethods specifies the list of allowed HTTP methods for CORS requests.
	// This controls which HTTP methods browsers are allowed to use in
	// cross-origin requests. Common methods include GET, POST, PUT, DELETE, OPTIONS.
	// Default: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
	AllowedMethods []string `yaml:"allowed_methods" default:"[\"GET\",\"POST\",\"PUT\",\"DELETE\",\"OPTIONS\"]" desc:"List of allowed HTTP methods." env:"ALLOWED_METHODS"`

	// AllowedHeaders specifies the list of allowed request headers for CORS requests.
	// This controls which headers browsers are allowed to send in cross-origin requests.
	// Common headers include Origin, Accept, Content-Type, Authorization.
	// Default: ["Origin", "Accept", "Content-Type", "X-Requested-With", "Authorization"]
	AllowedHeaders []string `yaml:"allowed_headers" default:"[\"Origin\",\"Accept\",\"Content-Type\",\"X-Requested-With\",\"Authorization\"]" desc:"List of allowed request headers." env:"ALLOWED_HEADERS"`

	// AllowCredentials determines whether cookies, authorization headers,
	// and TLS client certificates are allowed in CORS requests.
	// Set to true when your API needs to handle authenticated cross-origin requests.
	// Default: false
	AllowCredentials bool `yaml:"allow_credentials" default:"false" desc:"Allow credentials in CORS requests." env:"ALLOW_CREDENTIALS"`

	// MaxAge specifies the maximum age for CORS preflight cache in seconds.
	// This controls how long browsers can cache preflight request results,
	// reducing the number of preflight requests for repeated cross-origin calls.
	// Default: 300 (5 minutes)
	MaxAge int `yaml:"max_age" default:"300" desc:"Maximum age for CORS preflight cache in seconds." env:"MAX_AGE"`

	// Timeout specifies the default request timeout.
	// This sets a default timeout for request processing, though individual
	// handlers may override this with their own timeout logic.
	// Default: 60s (60 seconds)
	Timeout time.Duration `yaml:"timeout" desc:"Default request timeout." env:"TIMEOUT"`

	// BasePath specifies a base path prefix for all routes registered through this module.
	// When set, all routes will be prefixed with this path. Useful for mounting
	// the application under a sub-path or for API versioning.
	// Example: "/api/v1" would make a route "/users" accessible as "/api/v1/users"
	// Default: "" (no prefix)
	BasePath string `yaml:"basepath" desc:"A base path prefix for all routes registered through this module." env:"BASE_PATH"`
}

// Validate implements the modular.ConfigValidator interface.
// This method is called during configuration loading to ensure
// the configuration values are valid and consistent.
//
// Currently performs basic validation but can be extended to include:
//   - URL validation for allowed origins
//   - Timeout range validation
//   - Base path format validation
func (c *ChiMuxConfig) Validate() error {
	// Add custom validation logic here
	return nil
}
