package modular

import (
	"fmt"
	"github.com/golobby/config/v3"
	"reflect"
)

// LoadAppConfigFunc is the function type for loading application configuration
type LoadAppConfigFunc func(*StdApplication) error

// AppConfigLoader is the default implementation that can be replaced in tests
var AppConfigLoader LoadAppConfigFunc = loadAppConfig

type ConfigProvider interface {
	GetConfig() any
}

type StdConfigProvider struct {
	cfg any
}

func (s *StdConfigProvider) GetConfig() any {
	return s.cfg
}

func NewStdConfigProvider(cfg any) *StdConfigProvider {
	return &StdConfigProvider{cfg: cfg}
}

type Config struct {
	*config.Config
	StructKeys map[string]interface{}
}

func NewConfig() *Config {
	return &Config{
		Config:     config.New(),
		StructKeys: make(map[string]interface{}),
	}
}

func (c *Config) AddStructKey(key string, target interface{}) *Config {
	c.StructKeys[key] = target
	return c
}

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
				return fmt.Errorf("%w", ErrConfigFeederError)
			}
		}

		if setupable, ok := target.(ConfigSetup); ok {
			if err := setupable.Setup(); err != nil {
				return fmt.Errorf("%w for %s", ErrConfigSetupError, key)
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

	// Process main app config and all sections
	hasConfigs := false
	tempConfigs := make(map[string]configInfo)

	// Process main app config if provided
	if app.cfgProvider != nil {
		mainCfg := app.cfgProvider.GetConfig()
		if mainCfg == nil {
			app.logger.Warn("Main config is nil, skipping main config loading")
		} else {
			tempMainCfg, mainCfgInfo, err := createTempConfig(mainCfg)
			if err != nil {
				return fmt.Errorf("failed to create temp config: %w", err)
			}

			cfgBuilder.AddStruct(tempMainCfg)
			tempConfigs["main"] = mainCfgInfo
			hasConfigs = true

			app.logger.Debug("Added main config for loading", "type", reflect.TypeOf(mainCfg))
		}
	}

	// Process registered sections
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
			return fmt.Errorf("failed to create temp config for section %s: %w", sectionKey, err)
		}

		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		tempConfigs[sectionKey] = sectionInfo
		hasConfigs = true

		app.logger.Debug("Added section config for loading", "section", sectionKey, "type", reflect.TypeOf(sectionCfg))
	}

	// If no valid configs found, return early
	if !hasConfigs {
		app.logger.Info("No valid configs found, skipping config loading")
		return nil
	}

	// Feed all configs at once
	if err := cfgBuilder.Feed(); err != nil {
		return err
	}

	// Update all configs with newly populated values
	if mainInfo, exists := tempConfigs["main"]; exists {
		updateConfig(app, &app.cfgProvider, mainInfo)
		app.logger.Debug("Updated main config")
	}

	// Update section configs
	for sectionKey, info := range tempConfigs {
		if sectionKey == "main" {
			continue
		}
		updateSectionConfig(app, sectionKey, info)
		app.logger.Debug("Updated section config", "section", sectionKey)
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

func updateConfig(app *StdApplication, provider *ConfigProvider, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Debug("Creating new provider with updated config (original was non-pointer)")
		*provider = NewStdConfigProvider(info.tempVal.Elem().Interface())
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
