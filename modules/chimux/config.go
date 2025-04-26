package chimux

// ChiMuxConfig holds the configuration for the chimux module
type ChiMuxConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" default:"[\"*\"]" desc:"List of allowed origins for CORS requests."`                                                               // List of allowed origins for CORS requests.
	AllowedMethods   []string `yaml:"allowed_methods" default:"[\"GET\",\"POST\",\"PUT\",\"DELETE\",\"OPTIONS\"]" desc:"List of allowed HTTP methods."`                                  // List of allowed HTTP methods.
	AllowedHeaders   []string `yaml:"allowed_headers" default:"[\"Origin\",\"Accept\",\"Content-Type\",\"X-Requested-With\",\"Authorization\"]" desc:"List of allowed request headers."` // List of allowed request headers.
	AllowCredentials bool     `yaml:"allow_credentials" default:"false" desc:"Allow credentials in CORS requests."`                                                                      // Allow credentials in CORS requests.
	MaxAge           int      `yaml:"max_age" default:"300" desc:"Maximum age for CORS preflight cache in seconds."`                                                                     // Maximum age for CORS preflight cache in seconds.
	Timeout          int      `yaml:"timeout" default:"60000" desc:"Default request timeout."`                                                                                           // Default request timeout.
	BasePath         string   `yaml:"basepath" desc:"A base path prefix for all routes registered through this module."`                                                                 // A base path prefix for all routes registered through this module.
}

// Validate implements the modular.ConfigValidator interface
func (c *ChiMuxConfig) Validate() error {
	// Add custom validation logic here
	return nil
}
