package modular

import (
	"fmt"
	"github.com/golobby/config/v3"
	"reflect"
)

// LoadAppConfigFunc is the function type for loading application configuration
type LoadAppConfigFunc func(*Application) error

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
		return err
	}

	for key, target := range c.StructKeys {
		for _, f := range c.Feeders {
			cf, ok := f.(ComplexFeeder)
			if !ok {
				continue
			}

			if err := cf.FeedKey(key, target); err != nil {
				return fmt.Errorf("config: feeder error: %v", err)
			}
		}

		if setupable, ok := target.(ConfigSetup); ok {
			if err := setupable.Setup(); err != nil {
				return fmt.Errorf("config: setup error for %s: %v", key, err)
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

func loadAppConfig(app *Application) error {
	// Guard against nil application
	if app == nil {
		return fmt.Errorf("application is nil")
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

	// Handle main app config if provided
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

			// Feed the config
			if err := cfgBuilder.Feed(); err != nil {
				return err
			}

			// Update main app config
			updateConfig(app, &app.cfgProvider, mainCfgInfo)
		}
	}

	// Process registered sections
	sectionInfos := make(map[string]configInfo)
	hasValidSections := false

	// Create temporary structs for all registered sections
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

		sectionInfos[sectionKey] = sectionInfo
		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
		hasValidSections = true
	}

	// Feed the config for sections if any exist
	if hasValidSections {
		if err := cfgBuilder.Feed(); err != nil {
			return err
		}

		// Update all section configs
		for sectionKey, info := range sectionInfos {
			updateSectionConfig(app, sectionKey, info)
		}
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
// Returns error if cfg is nil
func createTempConfig(cfg any) (interface{}, configInfo, error) {
	if cfg == nil {
		return nil, configInfo{}, fmt.Errorf("cannot create temp config: config is nil")
	}

	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Ptr

	var targetType reflect.Type
	if isPtr {
		if cfgValue.IsNil() {
			return nil, configInfo{}, fmt.Errorf("cannot create temp config: config pointer is nil")
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

func updateConfig(app *Application, provider *ConfigProvider, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Info("Creating new provider with updated config (original was non-pointer)")
		*provider = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}

func updateSectionConfig(app *Application, sectionKey string, info configInfo) {
	if info.isPtr {
		info.originalVal.Elem().Set(info.tempVal.Elem())
	} else {
		app.logger.Info("Creating new provider for section", "section", sectionKey)
		app.cfgSections[sectionKey] = NewStdConfigProvider(info.tempVal.Elem().Interface())
	}
}
