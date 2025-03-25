package modular

import (
	"errors"
	"fmt"
	"github.com/golobby/config/v3"
	"reflect"
	"strings"
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

func loadAppConfig(app *Application) error {
	// Get the config
	cfg := app.cfgProvider.GetConfig()

	// Determine if it's a pointer or a concrete value
	cfgValue := reflect.ValueOf(cfg)
	isPtr := cfgValue.Kind() == reflect.Ptr

	// Create a temporary config of the same type
	var targetType reflect.Type
	if isPtr {
		// It's a pointer, get the type it points to
		targetType = cfgValue.Elem().Type()
	} else {
		// It's a concrete value, use its type directly
		targetType = cfgValue.Type()
	}

	// Create new instance of same type
	tempCfgValue := reflect.New(targetType)
	tempCfg := tempCfgValue.Interface()

	// Build the configuration
	cfgBuilder := config.New()
	for _, feeder := range ConfigFeeders {
		cfgBuilder.AddFeeder(feeder)
	}
	cfgBuilder.AddStruct(tempCfg)

	if err := cfgBuilder.Feed(); err != nil {
		return err
	}

	// Update the original config based on its type
	if isPtr {
		// If original was a pointer, update what it points to
		cfgValue.Elem().Set(tempCfgValue.Elem())
	} else {
		// If it was a concrete value, replace the provider
		app.logger.Info("Creating new provider with updated config (original was non-pointer)")
		app.cfgProvider = NewStdConfigProvider(tempCfgValue.Elem().Interface())
	}

	// Handle module configs
	if err := processModuleConfigs(app); err != nil {
		return fmt.Errorf("failed to process module configs: %w", err)
	}

	return nil
}

func processModuleConfigs(app *Application) error {
	// Load raw config into a map to find module sections
	rawMap := make(map[string]interface{})
	cfgBuilder := config.New()
	for _, feeder := range ConfigFeeders {
		if _, ok := feeder.(EnvFeeder); ok {
			// skip env feeder
			continue
		}
		cfgBuilder.AddFeeder(feeder)
	}
	cfgBuilder.AddStruct(&rawMap)
	if err := cfgBuilder.Feed(); err != nil {
		err = fmt.Errorf("failed to feed module config: %w", err)
		return err
	}

	// Check each key for module config sections
	appCfg := app.cfgProvider.GetConfig()
	for key, value := range rawMap {
		// Skip fields that belong to app config
		if hasMatchingField(appCfg, key, "yaml") {
			continue
		}
		if hasMatchingField(appCfg, key, "toml") {
			continue
		}
		if hasMatchingField(appCfg, key, "json") {
			continue
		}

		// Check if this is a registered module config section
		if provider, exists := app.cfgSections[key]; exists {
			// Get the module's config
			modCfg := provider.GetConfig()

			// Check if the config is a pointer or value
			modCfgValue := reflect.ValueOf(modCfg)
			isPtr := modCfgValue.Kind() == reflect.Ptr

			// Create temporary config of the same type
			var targetType reflect.Type
			if isPtr {
				targetType = modCfgValue.Elem().Type()
			} else {
				targetType = modCfgValue.Type()
			}

			// Create new instance
			tempCfgValue := reflect.New(targetType)
			tempModCfg := tempCfgValue.Interface()

			// Process config data
			if valueMap, ok := value.(map[string]interface{}); ok {
				if err := mapToStruct(valueMap, tempModCfg); err != nil {
					return fmt.Errorf("failed to map module config to struct: %w", err)
				}
			}

			// Apply env variables if needed
			if hasEnvFeeder() || hasDotEnvFeeder() {
				// Create a builder just for this section
				sectionBuilder := config.New()
				for _, feeder := range ConfigFeeders {
					if _, ok := feeder.(EnvFeeder); ok {
						sectionBuilder.AddFeeder(feeder)
					}
					if _, ok := feeder.(DotEnvFeeder); ok {
						sectionBuilder.AddFeeder(feeder)
					}
				}
				sectionBuilder.AddStruct(tempModCfg)
				if err := sectionBuilder.Feed(); err != nil {
					err = fmt.Errorf("failed to feed module config section: %w", err)
					return err
				}
			}

			// Update the module config based on its type
			if isPtr {
				// If original was a pointer, update what it points to
				modCfgValue.Elem().Set(tempCfgValue.Elem())
			} else {
				// If it was a concrete value, replace the provider with a new one
				app.cfgSections[key] = NewStdConfigProvider(tempCfgValue.Elem().Interface())
			}
		}
	}

	return nil
}

func mapToStruct(data map[string]interface{}, target interface{}) error {
	for key, value := range data {
		if hasYamlFeeder() {
			if hasMatchingField(target, key, "yaml") {
				if err := setTaggedFieldValue(target, key, "yaml", value); err != nil {
					return err
				}
			}
		}
		if hasTomlFeeder() {
			if hasMatchingField(target, key, "toml") {
				if err := setTaggedFieldValue(target, key, "toml", value); err != nil {
					return err
				}
			}
		}
		if hasJsonFeeder() {
			if hasMatchingField(target, key, "json") {
				if err := setTaggedFieldValue(target, key, "json", value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Helper functions to detect feeder types
func hasYamlFeeder() bool {
	for _, f := range ConfigFeeders {
		if _, ok := f.(YamlFeeder); ok {
			return true
		}
	}
	return false
}

func hasTomlFeeder() bool {
	for _, f := range ConfigFeeders {
		if _, ok := f.(TomlFeeder); ok {
			return true
		}
	}
	return false
}

func hasJsonFeeder() bool {
	for _, f := range ConfigFeeders {
		if _, ok := f.(JsonFeeder); ok {
			return true
		}
	}
	return false
}

func hasEnvFeeder() bool {
	for _, f := range ConfigFeeders {
		if _, ok := f.(EnvFeeder); ok {
			return true
		}
	}
	return false
}

func hasDotEnvFeeder() bool {
	for _, f := range ConfigFeeders {
		if _, ok := f.(DotEnvFeeder); ok {
			return true
		}
	}
	return false
}

type dynamicConfig struct {
	appCfg    *any
	unmatched map[string]any `yaml:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *dynamicConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First unmarshal the standard fields
	if err := unmarshal(c.appCfg); err != nil {
		return err
	}

	// Then get all fields as a map to find unmatched sections
	var rawMap map[string]interface{}
	if err := unmarshal(&rawMap); err != nil {
		return err
	}

	c.unmatched = make(map[string]any)

	// Process all keys in the map
	for key, value := range rawMap {
		// Skip AppConfig fields by checking against struct tags
		if hasMatchingField(c.appCfg, key, "yaml") {
			continue
		}

		// Store unmatched section
		c.unmatched[key] = value
	}

	return nil
}

func hasMatchingField(cfg any, fieldName string, tag string) bool {
	// First, check if we have a pointer
	val := reflect.ValueOf(cfg)
	if val.Kind() == reflect.Ptr {
		// Dereference the pointer
		val = val.Elem()
	}

	// Now check if we have an interface
	if val.Kind() == reflect.Interface {
		// Get the concrete value inside the interface
		val = reflect.ValueOf(val.Interface())
	}

	// Check if it's a struct
	if val.Kind() != reflect.Struct {
		return false
	}

	// Now we can safely get the type and enumerate fields
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag != "" {
			fieldTag := field.Tag.Get(tag)
			if fieldTag == fieldName {
				return true
			}
		}
		// check if lowercase field name matches lowercase key
		if strings.ToLower(field.Name) == strings.ToLower(fieldName) {
			return true
		}
	}
	return false
}

var (
	ErrNotStruct     = errors.New("not a struct")
	ErrFieldNotFound = errors.New("field not found")
)

func setTaggedFieldValue(cfg any, fieldName string, tag string, value any) error {
	// First, check if we have a pointer
	val := reflect.ValueOf(cfg)
	if val.Kind() == reflect.Ptr {
		// Dereference the pointer
		val = val.Elem()
	}

	// Now check if we have an interface
	if val.Kind() == reflect.Interface {
		// Get the concrete value inside the interface
		val = reflect.ValueOf(val.Interface())
	}

	// Check if it's a struct
	if val.Kind() != reflect.Struct {
		return ErrNotStruct
	}

	// Now we can safely get the type and enumerate fields
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if tag != "" {
			fieldTag := field.Tag.Get(tag)
			if fieldTag == fieldName {
				// Set the value
				val.Field(i).Set(reflect.ValueOf(value))
				return nil
			}
		}
		// check if lowercase field name matches lowercase key
		if strings.ToLower(field.Name) == strings.ToLower(fieldName) {
			// Set the value
			val.Field(i).Set(reflect.ValueOf(value))
			return nil
		}
	}
	return ErrFieldNotFound
}
