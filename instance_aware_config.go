package modular

import (
	"reflect"
)

// InstanceAwareConfigProvider handles configuration for multiple instances of the same type
type InstanceAwareConfigProvider struct {
	cfg any
	instancePrefixFunc InstancePrefixFunc
}

// NewInstanceAwareConfigProvider creates a new instance-aware configuration provider
func NewInstanceAwareConfigProvider(cfg any, prefixFunc InstancePrefixFunc) *InstanceAwareConfigProvider {
	return &InstanceAwareConfigProvider{
		cfg: cfg,
		instancePrefixFunc: prefixFunc,
	}
}

// GetConfig returns the configuration object
func (p *InstanceAwareConfigProvider) GetConfig() any {
	return p.cfg
}

// GetInstancePrefixFunc returns the instance prefix function
func (p *InstanceAwareConfigProvider) GetInstancePrefixFunc() InstancePrefixFunc {
	return p.instancePrefixFunc
}

// InstanceAwareConfigSupport indicates that a configuration supports instance-aware feeding
type InstanceAwareConfigSupport interface {
	// GetInstanceConfigs returns a map of instance configurations that should be fed with instance-aware feeders
	GetInstanceConfigs() map[string]interface{}
}

// Enhanced configuration loading that handles instance-aware configurations
func loadAppConfigWithInstanceAwareness(app *StdApplication) error {
	// Guard against nil application
	if app == nil {
		return ErrApplicationNil
	}

	// Skip if no ConfigFeeders are defined
	if len(ConfigFeeders) == 0 {
		app.logger.Info("No config feeders defined, skipping config loading")
		return nil
	}

	// Build the configuration with instance-aware support
	cfgBuilder := NewConfig()
	instanceAwareFeeder := NewInstanceAwareEnvFeeder(nil) // Default instance-aware feeder

	// Add regular feeders
	for _, feeder := range ConfigFeeders {
		cfgBuilder.AddFeeder(feeder)
	}

	// Process configs with instance-awareness
	tempConfigs, hasConfigs := processConfigsWithInstanceAwareness(app, cfgBuilder, instanceAwareFeeder)

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

// processConfigsWithInstanceAwareness handles config collection with instance-aware support
func processConfigsWithInstanceAwareness(app *StdApplication, cfgBuilder *Config, instanceFeeder InstanceAwareFeeder) (map[string]configInfo, bool) {
	tempConfigs := make(map[string]configInfo)
	hasConfigs := false

	// Process main app config if provided
	if processedMain := processMainConfigWithInstanceAwareness(app, cfgBuilder, instanceFeeder, tempConfigs); processedMain {
		hasConfigs = true
	}

	// Process registered sections
	if processedSections := processSectionConfigsWithInstanceAwareness(app, cfgBuilder, instanceFeeder, tempConfigs); processedSections {
		hasConfigs = true
	}

	return tempConfigs, hasConfigs
}

// processMainConfigWithInstanceAwareness handles main config with instance-awareness
func processMainConfigWithInstanceAwareness(app *StdApplication, cfgBuilder *Config, instanceFeeder InstanceAwareFeeder, tempConfigs map[string]configInfo) bool {
	if app.cfgProvider == nil {
		return false
	}

	mainCfg := app.cfgProvider.GetConfig()
	if mainCfg == nil {
		app.logger.Warn("Main config is nil, skipping main config loading")
		return false
	}

	// Check if it's an instance-aware config provider
	if iaProvider, ok := app.cfgProvider.(*InstanceAwareConfigProvider); ok && iaProvider.instancePrefixFunc != nil {
		// Update instance feeder with the prefix function
		instanceFeeder = NewInstanceAwareEnvFeeder(iaProvider.instancePrefixFunc)
		cfgBuilder.AddFeeder(instanceFeeder)
	}

	// Handle instance-aware configuration
	if iaConfig, ok := mainCfg.(InstanceAwareConfigSupport); ok {
		instanceConfigs := iaConfig.GetInstanceConfigs()
		for instanceKey, instanceConfig := range instanceConfigs {
			if err := instanceFeeder.FeedKey(instanceKey, instanceConfig); err != nil {
				app.logger.Warn("Failed to feed instance config", "instance", instanceKey, "error", err)
			} else {
				app.logger.Debug("Fed instance config", "instance", instanceKey)
			}
		}
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

// processSectionConfigsWithInstanceAwareness handles section configs with instance-awareness
func processSectionConfigsWithInstanceAwareness(app *StdApplication, cfgBuilder *Config, instanceFeeder InstanceAwareFeeder, tempConfigs map[string]configInfo) bool {
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

		// Check if it's an instance-aware config provider
		currentInstanceFeeder := instanceFeeder
		if iaProvider, ok := provider.(*InstanceAwareConfigProvider); ok && iaProvider.instancePrefixFunc != nil {
			// Create a section-specific instance feeder
			currentInstanceFeeder = NewInstanceAwareEnvFeeder(iaProvider.instancePrefixFunc)
			cfgBuilder.AddFeeder(currentInstanceFeeder)
		}

		// Handle instance-aware configuration
		if iaConfig, ok := sectionCfg.(InstanceAwareConfigSupport); ok {
			instanceConfigs := iaConfig.GetInstanceConfigs()
			for instanceKey, instanceConfig := range instanceConfigs {
				if err := currentInstanceFeeder.FeedKey(instanceKey, instanceConfig); err != nil {
					app.logger.Warn("Failed to feed instance config", "section", sectionKey, "instance", instanceKey, "error", err)
				} else {
					app.logger.Debug("Fed instance config", "section", sectionKey, "instance", instanceKey)
				}
			}
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