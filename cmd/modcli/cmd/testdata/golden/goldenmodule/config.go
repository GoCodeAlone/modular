package goldenmodule

// GoldenModuleConfig holds the configuration for the GoldenModule module
type GoldenModuleConfig struct {
	ApiKey string `yaml:"apikey" json:"apikey" toml:"apikey" required:"true" desc:"API key for authentication"` // API key for authentication
	MaxConnections int `yaml:"maxconnections" json:"maxconnections" toml:"maxconnections" required:"true" default:"10" desc:"Maximum number of concurrent connections"` // Maximum number of concurrent connections
	Debug bool `yaml:"debug" json:"debug" toml:"debug" default:"false" desc:"Enable debug mode"` // Enable debug mode
}

// Validate implements the modular.ConfigValidator interface
func (c *GoldenModuleConfig) Validate() error {
	// Add custom validation logic here
	return nil
}
