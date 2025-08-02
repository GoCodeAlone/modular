package modular

import (
	"fmt"
	"reflect"
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
//
//	oldLoader := modular.AppConfigLoader
//	defer func() { modular.AppConfigLoader = oldLoader }()
//	modular.AppConfigLoader = func(app *StdApplication) error {
//	    // Custom test configuration loading
//	    return nil
//	}
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
//
//	type MyConfig struct {
//	    Host string `json:"host" default:"localhost"`
//	    Port int    `json:"port" default:"8080"`
//	}
//
//	cfg := &MyConfig{}
//	provider := modular.NewStdConfigProvider(cfg)
func NewStdConfigProvider(cfg any) *StdConfigProvider {
	return &StdConfigProvider{cfg: cfg}
}

// Config represents a configuration builder that can combine multiple feeders and structures.
// It provides functionality for the modular framework to coordinate configuration loading.
//
// The Config builder allows you to:
//   - Add multiple configuration sources (files, environment, etc.)
//   - Combine configuration from different feeders
//   - Apply configuration to multiple struct targets
//   - Track which structs have been configured
//   - Enable verbose debugging for configuration processing
//   - Track field-level population details
type Config struct {
	// Feeders contains all the registered configuration feeders
	Feeders []Feeder
	// StructKeys maps struct identifiers to their configuration objects.
	// Used internally to track which configuration structures have been processed.
	StructKeys map[string]interface{}
	// VerboseDebug enables detailed logging during configuration processing
	VerboseDebug bool
	// Logger is used for verbose debug logging
	Logger Logger
	// FieldTracker tracks which fields are populated by which feeders
	FieldTracker FieldTracker
}

// NewConfig creates a new configuration builder.
// The returned Config can be used to set up complex configuration scenarios
// involving multiple sources and target structures.
//
// Example:
//
//	cfg := modular.NewConfig()
//	cfg.AddFeeder(modular.ConfigFeeders[0]) // Add file feeder
//	cfg.AddStruct(&myConfig)                // Add target struct
//	err := cfg.Feed()                       // Load configuration
func NewConfig() *Config {
	return &Config{
		Feeders:      make([]Feeder, 0),
		StructKeys:   make(map[string]interface{}),
		VerboseDebug: false,
		Logger:       nil,
		FieldTracker: NewDefaultFieldTracker(),
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (c *Config) SetVerboseDebug(enabled bool, logger Logger) *Config {
	c.VerboseDebug = enabled
	c.Logger = logger

	// Set logger on field tracker
	if c.FieldTracker != nil {
		c.FieldTracker.SetLogger(logger)
	}

	// Apply verbose debugging to any verbose-aware feeders
	for _, feeder := range c.Feeders {
		if verboseFeeder, ok := feeder.(VerboseAwareFeeder); ok {
			verboseFeeder.SetVerboseDebug(enabled, logger)
		}
	}

	return c
}

// AddFeeder adds a configuration feeder to support verbose debugging and field tracking
func (c *Config) AddFeeder(feeder Feeder) *Config {
	c.Feeders = append(c.Feeders, feeder)

	// If verbose debugging is enabled, apply it to this feeder
	if c.VerboseDebug && c.Logger != nil {
		if verboseFeeder, ok := feeder.(VerboseAwareFeeder); ok {
			verboseFeeder.SetVerboseDebug(true, c.Logger)
		}
	}
	// If field tracking is enabled, apply it to this feeder
	if c.FieldTracker != nil {
		// Check for main package FieldTrackingFeeder interface
		if trackingFeeder, ok := feeder.(FieldTrackingFeeder); ok {
			trackingFeeder.SetFieldTracker(c.FieldTracker)
		} else {
			// Check for feeders package interface compatibility
			// Use reflection to check if the feeder has a SetFieldTracker method
			feederValue := reflect.ValueOf(feeder)
			setFieldTrackerMethod := feederValue.MethodByName("SetFieldTracker")
			if setFieldTrackerMethod.IsValid() {
				// Create a bridge adapter and call SetFieldTracker
				bridge := NewFieldTrackerBridge(c.FieldTracker)
				args := []reflect.Value{reflect.ValueOf(bridge)}
				setFieldTrackerMethod.Call(args)
			}
		}
	}

	return c
}

// AddStructKey adds a structure with a key to the configuration
func (c *Config) AddStructKey(key string, target interface{}) *Config {
	c.StructKeys[key] = target
	return c
}

// SetFieldTracker sets the field tracker for capturing field population details
func (c *Config) SetFieldTracker(tracker FieldTracker) *Config {
	c.FieldTracker = tracker
	if c.Logger != nil {
		c.FieldTracker.SetLogger(c.Logger)
	}

	// Apply field tracking to any tracking-aware feeders
	for _, feeder := range c.Feeders {
		if trackingFeeder, ok := feeder.(FieldTrackingFeeder); ok {
			trackingFeeder.SetFieldTracker(tracker)
		}
	}

	return c
}

// FeedWithModuleContext feeds a single configuration structure with module context information
// This allows module-aware feeders to customize their behavior based on the module name
func (c *Config) FeedWithModuleContext(target interface{}, moduleName string) error {
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Starting module-aware config feed", "targetType", reflect.TypeOf(target), "moduleName", moduleName, "feedersCount", len(c.Feeders))
	}

	for i, f := range c.Feeders {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Applying feeder with module context", "feederIndex", i, "feederType", fmt.Sprintf("%T", f), "moduleName", moduleName)
		}

		// Try module-aware feeder first if available
		if maf, ok := f.(ModuleAwareFeeder); ok {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Using ModuleAwareFeeder", "feederType", fmt.Sprintf("%T", f), "moduleName", moduleName)
			}
			if err := maf.FeedWithModuleContext(target, moduleName); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("ModuleAwareFeeder failed", "feederType", fmt.Sprintf("%T", f), "error", err)
				}
				return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
			}
		} else {
			// Fall back to regular Feed method for non-module-aware feeders
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Using regular Feed method", "feederType", fmt.Sprintf("%T", f))
			}
			if err := f.Feed(target); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Regular Feed method failed", "feederType", fmt.Sprintf("%T", f), "error", err)
				}
				return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
			}
		}

		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Feeder applied successfully", "feederType", fmt.Sprintf("%T", f))
		}
	}

	// Apply defaults and validate config
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Validating config", "moduleName", moduleName)
	}

	if err := ValidateConfig(target); err != nil {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Config validation failed", "moduleName", moduleName, "error", err)
		}
		return fmt.Errorf("config validation error for %s: %w", moduleName, err)
	}

	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Config validation succeeded", "moduleName", moduleName)
	}

	// Call Setup if implemented
	if setupable, ok := target.(ConfigSetup); ok {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Calling Setup for config", "moduleName", moduleName)
		}
		if err := setupable.Setup(); err != nil {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Config setup failed", "moduleName", moduleName, "error", err)
			}
			return fmt.Errorf("%w for %s: %w", ErrConfigSetupError, moduleName, err)
		}
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Config setup succeeded", "moduleName", moduleName)
		}
	}

	return nil
}

// Feed with validation applies defaults and validates configs after feeding
func (c *Config) Feed() error {
	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Starting config feed process", "structKeysCount", len(c.StructKeys), "feedersCount", len(c.Feeders))
	}

	// If we have struct keys, feed them directly with field tracking
	if len(c.StructKeys) > 0 {
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("Using enhanced feeding process with field tracking")
		}

		// Feed each struct key with each feeder
		for key, target := range c.StructKeys {
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Processing struct key", "key", key, "targetType", reflect.TypeOf(target))
			}

			for i, f := range c.Feeders {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Applying feeder to struct", "key", key, "feederIndex", i, "feederType", fmt.Sprintf("%T", f))
				}

				// Try module-aware feeder first if this is a section config (not main config)
				if key != mainConfigSection {
					if maf, ok := f.(ModuleAwareFeeder); ok {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("Using ModuleAwareFeeder for section", "key", key, "feederType", fmt.Sprintf("%T", f))
						}
						if err := maf.FeedWithModuleContext(target, key); err != nil {
							if c.VerboseDebug && c.Logger != nil {
								c.Logger.Debug("ModuleAwareFeeder Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
							}
							return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
						}
					} else {
						// Fall back to regular Feed method for non-module-aware feeders
						if err := f.Feed(target); err != nil {
							if c.VerboseDebug && c.Logger != nil {
								c.Logger.Debug("Regular Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
							}
							return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
						}
					}
				} else {
					// Use regular Feed method for main config
					if err := f.Feed(target); err != nil {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("Feeder Feed method failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
						}
						return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
					}
				}

				// Also try ComplexFeeder if available (for instance-aware feeders)
				if cf, ok := f.(ComplexFeeder); ok {
					if c.VerboseDebug && c.Logger != nil {
						c.Logger.Debug("Applying ComplexFeeder FeedKey", "key", key, "feederType", fmt.Sprintf("%T", f))
					}

					if err := cf.FeedKey(key, target); err != nil {
						if c.VerboseDebug && c.Logger != nil {
							c.Logger.Debug("ComplexFeeder FeedKey failed", "key", key, "feederType", fmt.Sprintf("%T", f), "error", err)
						}
						return fmt.Errorf("config feeder error: %w: %w", ErrConfigFeederError, err)
					}
				}

				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Feeder applied successfully", "key", key, "feederType", fmt.Sprintf("%T", f))
				}
			}

			// Apply defaults and validate config
			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Validating config for struct key", "key", key)
			}

			if err := ValidateConfig(target); err != nil {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Config validation failed", "key", key, "error", err)
				}
				return fmt.Errorf("config validation error for %s: %w", key, err)
			}

			if c.VerboseDebug && c.Logger != nil {
				c.Logger.Debug("Config validation succeeded", "key", key)
			}

			// Call Setup if implemented
			if setupable, ok := target.(ConfigSetup); ok {
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Calling Setup for config", "key", key)
				}
				if err := setupable.Setup(); err != nil {
					if c.VerboseDebug && c.Logger != nil {
						c.Logger.Debug("Config setup failed", "key", key, "error", err)
					}
					return fmt.Errorf("%w for %s: %w", ErrConfigSetupError, key, err)
				}
				if c.VerboseDebug && c.Logger != nil {
					c.Logger.Debug("Config setup succeeded", "key", key)
				}
			}
		}
	} else {
		// No struct keys configured - this means no explicit structures were added
		if c.VerboseDebug && c.Logger != nil {
			c.Logger.Debug("No struct keys configured - skipping feed process")
		}
	}

	if c.VerboseDebug && c.Logger != nil {
		c.Logger.Debug("Config feed process completed successfully")
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

	if app.IsVerboseConfig() {
		app.logger.Debug("Starting configuration loading process")
	}

	// Skip if no ConfigFeeders are defined
	if len(ConfigFeeders) == 0 {
		app.logger.Info("No config feeders defined, skipping config loading")
		return nil
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration feeders available", "count", len(ConfigFeeders))
		for i, feeder := range ConfigFeeders {
			app.logger.Debug("Config feeder registered", "index", i, "type", fmt.Sprintf("%T", feeder))
		}
	}

	// Build the configuration
	cfgBuilder := NewConfig()
	if app.IsVerboseConfig() {
		cfgBuilder.SetVerboseDebug(true, app.logger)
	}
	for _, feeder := range ConfigFeeders {
		cfgBuilder.AddFeeder(feeder)
		if app.IsVerboseConfig() {
			app.logger.Debug("Added config feeder to builder", "type", fmt.Sprintf("%T", feeder))
		}
	}

	// Process configs
	tempConfigs, hasConfigs := processConfigs(app, cfgBuilder)

	// If no valid configs found, return early
	if !hasConfigs {
		app.logger.Info("No valid configs found, skipping config loading")
		return nil
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration structures prepared for feeding", "count", len(tempConfigs))
	}

	// Feed all configs at once
	if err := cfgBuilder.Feed(); err != nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Configuration feeding failed", "error", err)
		}
		return err
	}

	// Apply instance-aware feeding for supported configurations AFTER regular feeding
	if err := applyInstanceAwareFeeding(app, tempConfigs); err != nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Instance-aware feeding failed", "error", err)
		}
		return err
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration feeding completed successfully")
	}

	// Apply updated configs
	applyConfigUpdates(app, tempConfigs)

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration loading process completed")
	}

	return nil
}

// processConfigs handles the collection and preparation of configs
func processConfigs(app *StdApplication, cfgBuilder *Config) (map[string]configInfo, bool) {
	tempConfigs := make(map[string]configInfo)
	hasConfigs := false

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing configuration sections")
	}

	// Process main app config if provided
	if processedMain := processMainConfig(app, cfgBuilder, tempConfigs); processedMain {
		hasConfigs = true
	}

	// Process registered sections
	if processedSections := processSectionConfigs(app, cfgBuilder, tempConfigs); processedSections {
		hasConfigs = true
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Configuration processing completed", "totalConfigs", len(tempConfigs), "hasValidConfigs", hasConfigs)
	}

	return tempConfigs, hasConfigs
}

// processMainConfig handles the main application config
func processMainConfig(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	if app.cfgProvider == nil {
		if app.IsVerboseConfig() {
			app.logger.Debug("Main config provider is nil, skipping main config")
		}
		return false
	}

	mainCfg := app.cfgProvider.GetConfig()
	if mainCfg == nil {
		app.logger.Warn("Main config is nil, skipping main config loading")
		return false
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing main configuration", "configType", reflect.TypeOf(mainCfg), "section", mainConfigSection)
	}

	tempMainCfg, mainCfgInfo, err := createTempConfig(mainCfg)
	if err != nil {
		app.logger.Warn("Failed to create temp config, skipping main config", "error", err)
		return false
	}

	cfgBuilder.AddStructKey(mainConfigSection, tempMainCfg)
	tempConfigs[mainConfigSection] = mainCfgInfo
	app.logger.Debug("Added main config for loading", "type", reflect.TypeOf(mainCfg))

	if app.IsVerboseConfig() {
		app.logger.Debug("Main configuration prepared for feeding", "section", mainConfigSection)
	}

	return true
}

// processSectionConfigs handles the section configs
func processSectionConfigs(app *StdApplication, cfgBuilder *Config, tempConfigs map[string]configInfo) bool {
	hasValidSections := false

	if app.IsVerboseConfig() {
		app.logger.Debug("Processing configuration sections", "totalSections", len(app.cfgSections))
	}

	for sectionKey, provider := range app.cfgSections {
		if app.IsVerboseConfig() {
			app.logger.Debug("Processing configuration section", "section", sectionKey, "providerType", fmt.Sprintf("%T", provider))
		}

		if provider == nil {
			app.logger.Warn("Skipping nil config provider", "section", sectionKey)
			continue
		}

		sectionCfg := provider.GetConfig()
		if sectionCfg == nil {
			app.logger.Warn("Skipping section with nil config", "section", sectionKey)
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Section config retrieved", "section", sectionKey, "configType", reflect.TypeOf(sectionCfg))
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

		if app.IsVerboseConfig() {
			app.logger.Debug("Section configuration prepared for feeding", "section", sectionKey)
		}
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Section configuration processing completed", "validSections", hasValidSections)
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

// applyInstanceAwareFeeding applies instance-aware feeding to configurations that support it
func applyInstanceAwareFeeding(app *StdApplication, tempConfigs map[string]configInfo) error {
	if app.IsVerboseConfig() {
		app.logger.Debug("Starting instance-aware feeding process")
	}

	// Check each section for instance-aware config support
	for sectionKey := range tempConfigs {
		if sectionKey == mainConfigSection {
			continue // Skip main config section for now
		}

		// Get the original provider to check if it's instance-aware
		provider, exists := app.cfgSections[sectionKey]
		if !exists {
			continue
		}

		// Check if the provider is instance-aware
		iaProvider, isInstanceAware := provider.(*InstanceAwareConfigProvider)
		if !isInstanceAware {
			if app.IsVerboseConfig() {
				app.logger.Debug("Section provider is not instance-aware, skipping", "section", sectionKey)
			}
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Processing instance-aware section", "section", sectionKey)
		}

		// Get the config from the temporary config that was just fed with YAML/ENV data
		configInfo := tempConfigs[sectionKey]
		var tempConfig interface{}
		if configInfo.isPtr {
			tempConfig = configInfo.tempVal.Interface()
		} else {
			tempConfig = configInfo.tempVal.Elem().Interface()
		}

		// Check if it supports instance configurations
		instanceSupport, supportsInstances := tempConfig.(InstanceAwareConfigSupport)
		if !supportsInstances {
			if app.IsVerboseConfig() {
				app.logger.Debug("Config does not support instances, skipping", "section", sectionKey)
			}
			continue
		}

		// Get the instance configurations
		instances := instanceSupport.GetInstanceConfigs()
		if len(instances) == 0 {
			if app.IsVerboseConfig() {
				app.logger.Debug("No instances found for section", "section", sectionKey)
			}
			continue
		}

		if app.IsVerboseConfig() {
			app.logger.Debug("Found instances for section", "section", sectionKey, "instanceCount", len(instances))
		}

		// Get the prefix function
		prefixFunc := iaProvider.GetInstancePrefixFunc()
		if prefixFunc == nil {
			app.logger.Warn("Instance-aware provider missing prefix function", "section", sectionKey)
			continue
		}

		// Create instance-aware feeder
		instanceFeeder := NewInstanceAwareEnvFeeder(prefixFunc)

		// Apply verbose debug if enabled
		if app.IsVerboseConfig() {
			if verboseFeeder, ok := instanceFeeder.(VerboseAwareFeeder); ok {
				verboseFeeder.SetVerboseDebug(true, app.logger)
			}
		}

		// Feed each instance
		for instanceKey, instanceConfig := range instances {
			if app.IsVerboseConfig() {
				app.logger.Debug("Feeding instance configuration", "section", sectionKey, "instance", instanceKey)
			}

			if err := instanceFeeder.FeedKey(instanceKey, instanceConfig); err != nil {
				app.logger.Warn("Failed to feed instance configuration",
					"section", sectionKey, "instance", instanceKey, "error", err)
				continue
			}

			if app.IsVerboseConfig() {
				app.logger.Debug("Successfully fed instance configuration", "section", sectionKey, "instance", instanceKey)
			}
		}
	}

	if app.IsVerboseConfig() {
		app.logger.Debug("Instance-aware feeding process completed")
	}

	return nil
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
