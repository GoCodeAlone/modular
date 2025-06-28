package modular

import (
	"fmt"
	"reflect"

	"github.com/golobby/config/v3"
)

const mainConfigSection = "_main"

// LoadAppConfigFunc is the function type for loading application configuration.
// This function is responsible for loading configuration data into the application
// using the registered config feeders and config sections.
//
// The default implementation can be replaced for testing or custom configuration scenarios.
type LoadAppConfigFunc func(*StdApplication) error

// AppConfigLoader is the default implementation that can be replaced in tests.
// This variable allows the configuration loading strategy to be customized,
// which is particularly useful for testing scenarios where you want to
// control how configuration is loaded.
//
// Example of replacing for tests:
//   oldLoader := modular.AppConfigLoader
//   defer func() { modular.AppConfigLoader = oldLoader }()
//   modular.AppConfigLoader = func(app *StdApplication) error {
//       // Custom test configuration loading
//       return nil
//   }
var AppConfigLoader LoadAppConfigFunc = loadAppConfig

// ConfigProvider defines the interface for providing configuration objects.
// Configuration providers encapsulate configuration data and make it available
// to modules and the application framework.
//
// The framework supports multiple configuration sources (files, environment variables,
// command-line flags) and formats (JSON, YAML, TOML) through different providers.
type ConfigProvider interface {
	// GetConfig returns the configuration object.
	// The returned value should be a pointer to a struct that represents
	// the configuration schema. Modules typically type-assert this to
	// their expected configuration type.
	//
	// Example:
	//   cfg := provider.GetConfig().(*MyModuleConfig)
	GetConfig() any
}

// StdConfigProvider provides a standard implementation of ConfigProvider.
// It wraps a configuration struct and makes it available through the ConfigProvider interface.
//
// This is the most common way to create configuration providers for modules.
// Simply create your configuration struct and wrap it with NewStdConfigProvider.
type StdConfigProvider struct {
	cfg any
}

// GetConfig returns the configuration object.
// The returned value is the exact object that was passed to NewStdConfigProvider.
func (s *StdConfigProvider) GetConfig() any {
	return s.cfg
}

// NewStdConfigProvider creates a new standard configuration provider.
// The cfg parameter should be a pointer to a struct that defines the
// configuration schema for your module.
//
// Example:
//   type MyConfig struct {
//       Host string `json:"host" default:"localhost"`
//       Port int    `json:"port" default:"8080"`
//   }
//   
//   cfg := &MyConfig{}
//   provider := modular.NewStdConfigProvider(cfg)
func NewStdConfigProvider(cfg any) *StdConfigProvider {
	return &StdConfigProvider{cfg: cfg}
}

// Config represents a configuration builder that can combine multiple feeders and structures.
// It extends the golobby/config library with additional functionality for the modular framework.
//
// The Config builder allows you to:
//   - Add multiple configuration sources (files, environment, etc.)
//   - Combine configuration from different feeders
//   - Apply configuration to multiple struct targets
//   - Track which structs have been configured
type Config struct {
	*config.Config
	// StructKeys maps struct identifiers to their configuration objects.
	// Used internally to track which configuration structures have been processed.
	StructKeys map[string]interface{}
}

// NewConfig creates a new configuration builder.
// The returned Config can be used to set up complex configuration scenarios
// involving multiple sources and target structures.
//
// Example:
//   cfg := modular.NewConfig()
//   cfg.AddFeeder(modular.ConfigFeeders[0]) // Add file feeder
//   cfg.AddStruct(&myConfig)                // Add target struct
//   err := cfg.Feed()                       // Load configuration
func NewConfig() *Config {
	return &Config{
		Config:     config.New(),
		StructKeys: make(map[string]interface{}),
	}
}

// AddStructKey adds a structure with a key to the configuration
func (c *Config) AddStructKey(key string, target interface{}) *Config {
	c.StructKeys[key] = target
	return c
}

// Feed with validation applies defaults and validates configs after feeding
func (c *Config) Feed() error {
	if err := c.Config.Feed(); err != nil {
		return fmt.Errorf("config feed error: %w", err)
	}

	for key, target := range c.StructKeys {
		for _, f := range c.Feeders {
			cf, ok := f.(ComplexFeeder)
			if !ok {
				continue
			}

			if err := cf.FeedKey(key, target); err != nil {
				return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
			}
		}

		// Apply defaults and validate config
		if err := ValidateConfig(target); err != nil {
			return fmt.Errorf("config validation error for %s: %w", key, err)
		}

		// Call Setup if implemented
		if setupable, ok := target.(ConfigSetup); ok {
			if err := setupable.Setup(); err != nil {
				return fmt.Errorf("%w for %s: %w", ErrConfigSetupError, key, err)
			}
		}
	}

	return nil
}

// ConfigSetup is an interface that configs can implement
// to perform additional setup after being populated by feeders
type ConfigSetup interface {
	Setup() error
}

func loadAppConfig(app *StdApplication) error {
	// Guard against nil application
	if app == nil {
		return ErrApplicationNil
	}

	// Skip if no ConfigFeeders are defined
	if len(ConfigFeeders) == 0 {
		app.logger.Info("No config feeders defined, skipping config loading")
		return nil
	}

	// Build the configuration
	cfgBuilder := NewConfig()
	for _, feeder := range ConfigFeeders {
		cfgBuilder.AddFeeder(feeder)
	}

	// Process configs
	tempConfigs, hasConfigs := processConfigs(app, cfgBuilder)

	// If no valid configs found, return early
	if !hasConfigs {
		app.logger.Info("No valid configs found, skipping config loading")
		return nil
	}

	// Feed all configs at once
	if err := cfgBuilder.Feed(); err != nil {
		return err
	}

	// Apply updated configs
	applyConfigUpdates(app, tempConfigs)

	return nil
}

// processConfigs handles the collection and preparation of configs
func processConfigs(app *StdApplication, cfgBuilder *Config) (map[string]configInfo, bool) {
	tempConfigs := make(map[string]configInfo)
	hasConfigs := false

	// Process main app config if provided
	if processedMain := processMainConfig(app, cfgBuilder, tempConfigs); processedMain {
		hasConfigs = true
	}

	// Process registered sections
	if processedSections := processSectionConfigs(app, cfgBuilder, tempConfigs); processedSections {
		hasConfigs = true
	}

	return tempConfigs, hasConfigs
}

// processMainConfig handles the main application config
func processMainConfig(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	if app.cfgProvider == nil {
		return false
	}

	mainCfg := app.cfgProvider.GetConfig()
	if mainCfg == nil {
		app.logger.Warn("Main config is nil, skipping main config loading")
		return false
	}

	tempMainCfg, mainCfgInfo, err := createTempConfig(mainCfg)
	if err != nil {
		app.logger.Warn("Failed to create temp config, skipping main config", "error", err)
		return false
	}

	cfgBuilder.AddStruct(tempMainCfg)
	tempConfigs[mainConfigSection] = mainCfgInfo
	app.logger.Debug("Added main config for loading", "type", reflect.TypeOf(mainCfg))

	return true
}

// processSectionConfigs handles the section configs
func processSectionConfigs(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	hasValidSections := false

	for sectionKey, provider := range app.cfgSections {
		if provider == nil {
			app.logger.Warn("Skipping nil config provider", "section", sectionKey)
			continue
		}

		sectionCfg := provider.GetConfig()
		if sectionCfg == nil {
			app.logger.Warn("Skipping section with nil config", "section", sectionKey)
			continue
		}

		tempSectionCfg, sectionInfo, err := createTempConfig(sectionCfg)
		if err != nil {
			app.logger.Warn("Failed to create temp config for section, skipping",
				"section", sectionKey, "error", err)
			continue
		}

		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		tempConfigs[sectionKey] = sectionInfo
		hasValidSections = true

		app.logger.Debug("Added section config for loading",
			"section", sectionKey, "type", reflect.TypeOf(sectionCfg))
	}

	return hasValidSections
}

// applyConfigUpdates applies updates to all configs
func applyConfigUpdates(app *StdApplication, tempConfigs map[string]configInfo) {
	// Update main config if it exists
	if mainInfo, exists := tempConfigs[mainConfigSection]; exists {
		updateConfig(app, mainInfo)
		app.logger.Debug("Updated main config")
	}

	// Update section configs
	for sectionKey, info := range tempConfigs {
		if sectionKey == mainConfigSection {
			continue
		}
		updateSectionConfig(app, sectionKey, info)
		app.logger.Debug("Updated section config", "section", sectionKey)
	}
}

// Helper types and functions
type configInfo struct {
	originalVal reflect.Value
	tempVal     reflect.Value
	isPtr       bool
}

// createTempConfig creates a temporary config for feeding values
func createTempConfig(cfg any) (interface{}, configInfo, error) {
	if cfg == nil {
		return nil, configInfo{}, ErrConfigNil
	}

	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Ptr

	var targetType reflect.Type
	if isPtr {
		if cfgValue.IsNil() {
			return nil, configInfo{}, ErrConfigNilPointer
		}
		targetType = cfgValue.Elem().Type()
	} else {
		targetType = cfgValue.Type()
	}

	tempCfgValue := reflect.New(targetType)

	return tempCfgValue.Interface(), configInfo{
		originalVal: cfgValue,
		tempVal:     tempCfgValue,
		isPtr:       isPtr,
	}, nil
}

func updateConfig(app *StdApplication, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Debug("Creating new provider with updated config (original was non-pointer)")
		// For non-pointer configs, we need to update the provider reference
		app.cfgProvider = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}

func updateSectionConfig(app *StdApplication, sectionKey string, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Debug("Creating new provider for section", "section", sectionKey)
		app.cfgSections[sectionKey] = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}
