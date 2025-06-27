package chimux

// ChiMuxConfig holds the configuration for the chimux module
type ChiMuxConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" default:"[\"*\"]" desc:"List of allowed origins for CORS requests." env:"ALLOWED_ORIGINS"`                                                               // List of allowed origins for CORS requests.
	AllowedMethods   []string `yaml:"allowed_methods" default:"[\"GET\",\"POST\",\"PUT\",\"DELETE\",\"OPTIONS\"]" desc:"List of allowed HTTP methods." env:"ALLOWED_METHODS"`                                  // List of allowed HTTP methods.
	AllowedHeaders   []string `yaml:"allowed_headers" default:"[\"Origin\",\"Accept\",\"Content-Type\",\"X-Requested-With\",\"Authorization\"]" desc:"List of allowed request headers." env:"ALLOWED_HEADERS"` // List of allowed request headers.
	AllowCredentials bool     `yaml:"allow_credentials" default:"false" desc:"Allow credentials in CORS requests." env:"ALLOW_CREDENTIALS"`                                                                    // Allow credentials in CORS requests.
	MaxAge           int      `yaml:"max_age" default:"300" desc:"Maximum age for CORS preflight cache in seconds." env:"MAX_AGE"`                                                                             // Maximum age for CORS preflight cache in seconds.
	Timeout          int      `yaml:"timeout" default:"60000" desc:"Default request timeout." env:"TIMEOUT"`                                                                                                   // Default request timeout.
	BasePath         string   `yaml:"basepath" desc:"A base path prefix for all routes registered through this module." env:"BASE_PATH"`                                                                       // A base path prefix for all routes registered through this module.
}

// Validate implements the modular.ConfigValidator interface
func (c *ChiMuxConfig) Validate() error {
	// Add custom validation logic here
	return nil
}
