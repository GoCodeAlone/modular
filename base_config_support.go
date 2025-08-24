package modular

import (
	"os"

	"github.com/GoCodeAlone/modular/feeders"
)

// BaseConfigOptions holds configuration for base config support
type BaseConfigOptions struct {
	// ConfigDir is the root directory containing base/ and environments/ subdirectories
	ConfigDir string
	// Environment specifies which environment overrides to apply (e.g., "prod", "staging", "dev")
	Environment string
	// Enabled determines whether base config support is active
	Enabled bool
}

// BaseConfigSettings holds the global base config settings
var BaseConfigSettings BaseConfigOptions

// SetBaseConfig configures the framework to use base configuration with environment overrides
// This should be called before building the application if you want to use base config support
func SetBaseConfig(configDir, environment string) {
	BaseConfigSettings = BaseConfigOptions{
		ConfigDir:   configDir,
		Environment: environment,
		Enabled:     true,
	}
}

// IsBaseConfigEnabled returns true if base configuration support is enabled
func IsBaseConfigEnabled() bool {
	return BaseConfigSettings.Enabled
}

// DetectBaseConfigStructure automatically detects if base configuration structure exists
// and enables it if found. This is called automatically during application initialization.
func DetectBaseConfigStructure() bool {
	// Check common config directory locations
	configDirs := []string{
		"config",
		"configs",
		".",
	}

	for _, configDir := range configDirs {
		if feeders.IsBaseConfigStructure(configDir) {
			// Try to determine environment from environment variable or use "dev" as default
			environment := os.Getenv("APP_ENVIRONMENT")
			if environment == "" {
				environment = os.Getenv("ENVIRONMENT")
			}
			if environment == "" {
				environment = os.Getenv("ENV")
			}
			if environment == "" {
				// Check if we can find any environments
				environments := feeders.GetAvailableEnvironments(configDir)
				if len(environments) > 0 {
					// Use the first environment alphabetically for deterministic behavior
					environment = environments[0] // environments is already sorted by GetAvailableEnvironments
				} else {
					environment = "dev"
				}
			}

			SetBaseConfig(configDir, environment)
			return true
		}
	}

	return false
}

// GetBaseConfigFeeder returns a BaseConfigFeeder if base config is enabled
func GetBaseConfigFeeder() feeders.Feeder {
	if !BaseConfigSettings.Enabled {
		return nil
	}

	return feeders.NewBaseConfigFeeder(BaseConfigSettings.ConfigDir, BaseConfigSettings.Environment)
}

// GetBaseConfigComplexFeeder returns a BaseConfigFeeder as ComplexFeeder if base config is enabled
func GetBaseConfigComplexFeeder() ComplexFeeder {
	if !BaseConfigSettings.Enabled {
		return nil
	}

	feeder := feeders.NewBaseConfigFeeder(BaseConfigSettings.ConfigDir, BaseConfigSettings.Environment)
	return feeder
}
