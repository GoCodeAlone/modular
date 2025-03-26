package modular

import (
	"fmt"
	"github.com/golobby/config/v3"
	"reflect"
)

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

		// TODO: need a setup struct for module configs
	}

	return nil
}

func loadAppConfig(app *Application) error {
	// Build the configuration
	cfgBuilder := NewConfig()
	for _, feeder := range ConfigFeeders {
		cfgBuilder.AddFeeder(feeder)
	}

	// Handle main app config
	tempMainCfg, mainCfgInfo := createTempConfig(app.cfgProvider.GetConfig())
	cfgBuilder.AddStruct(tempMainCfg)

	// Create temporary structs for all registered sections
	sectionInfos := make(map[string]configInfo)
	for sectionKey, provider := range app.cfgSections {
		tempSectionCfg, sectionInfo := createTempConfig(provider.GetConfig())
		sectionInfos[sectionKey] = sectionInfo
		cfgBuilder.AddStructKey(sectionKey, tempSectionCfg)
	}

	// Feed the config
	if err := cfgBuilder.Feed(); err != nil {
		return err
	}

	// Update main app config
	updateConfig(app, &app.cfgProvider, tempMainCfg, mainCfgInfo)

	// Update all section configs
	for sectionKey, info := range sectionInfos {
		updateSectionConfig(app, sectionKey, info)
	}

	return nil
}

// Helper types and functions
type configInfo struct {
	originalVal reflect.Value
	tempVal     reflect.Value
	isPtr       bool
}

func createTempConfig(cfg any) (interface{}, configInfo) {
	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Ptr

	var targetType reflect.Type
	if isPtr {
		targetType = cfgValue.Elem().Type()
	} else {
		targetType = cfgValue.Type()
	}

	tempCfgValue := reflect.New(targetType)

	return tempCfgValue.Interface(), configInfo{
		originalVal: cfgValue,
		tempVal:     tempCfgValue,
		isPtr:       isPtr,
	}
}

func updateConfig(app *Application, provider *ConfigProvider, tempCfg interface{}, info configInfo) {
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
