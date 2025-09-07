// Package config provides configuration loading and management services
package config

import (
	"context"
	"errors"
	"reflect"
	"strconv"
)

// Static errors for configuration package
var (
	ErrLoadNotImplemented           = errors.New("load method not yet implemented")
	ErrReloadNotImplemented         = errors.New("reload method not yet implemented")
	ErrValidateNotImplemented       = errors.New("validate method not yet implemented")
	ErrProvenanceNotImplemented     = errors.New("provenance method not yet implemented")
	ErrStructValidateNotImplemented = errors.New("struct validation not yet implemented")
	ErrFieldValidateNotImplemented  = errors.New("field validation not yet implemented")
	ErrStartWatchNotImplemented     = errors.New("start watch method not yet implemented")
	ErrStopWatchNotImplemented      = errors.New("stop watch method not yet implemented")
	ErrConfigTypeNotFound           = errors.New("config type not found")
)

// Loader implements the ConfigLoader interface with basic stub functionality
type Loader struct {
	sources    []*ConfigSource
	validators []ConfigValidator
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		sources:    make([]*ConfigSource, 0),
		validators: make([]ConfigValidator, 0),
	}
}

// Load loads configuration from all configured sources and applies validation
func (l *Loader) Load(ctx context.Context, config interface{}) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}
	
	// Apply configuration loading from all sources in priority order
	// Sort sources by priority (higher priority first)
	sortedSources := make([]*ConfigSource, len(l.sources))
	copy(sortedSources, l.sources)
	
	// Simple bubble sort by priority (higher first)
	for i := 0; i < len(sortedSources)-1; i++ {
		for j := 0; j < len(sortedSources)-i-1; j++ {
			if sortedSources[j].Priority < sortedSources[j+1].Priority {
				sortedSources[j], sortedSources[j+1] = sortedSources[j+1], sortedSources[j]
			}
		}
	}

	// TODO: Load from actual sources, for now just apply defaults and validate
	err := l.applyDefaults(config)
	if err != nil {
		return err
	}

	// Validate the configuration
	err = l.Validate(ctx, config)
	if err != nil {
		return err
	}

	return nil
}

// Reload reloads configuration from sources, applying hot-reload logic where supported
func (l *Loader) Reload(ctx context.Context, config interface{}) error {
	// TODO: Implement configuration reloading
	return ErrReloadNotImplemented
}

// Validate validates the given configuration against defined rules and schemas
func (l *Loader) Validate(ctx context.Context, config interface{}) error {
	// Validate using all configured validators
	for _, validator := range l.validators {
		err := validator.ValidateStruct(ctx, config)
		if err != nil {
			return err
		}
	}

	// Built-in validation: check required fields using reflection
	err := l.validateRequiredFields(config)
	if err != nil {
		return err
	}

	return nil
}

// GetProvenance returns field-level provenance information for configuration
func (l *Loader) GetProvenance(ctx context.Context, fieldPath string) (*FieldProvenance, error) {
	// TODO: Implement provenance tracking
	return nil, ErrProvenanceNotImplemented
}

// GetSources returns information about all configured configuration sources
func (l *Loader) GetSources(ctx context.Context) ([]*ConfigSource, error) {
	// TODO: Return actual configured sources
	return l.sources, nil
}

// AddSource adds a configuration source to the loader
func (l *Loader) AddSource(source *ConfigSource) {
	l.sources = append(l.sources, source)
}

// AddValidator adds a configuration validator to the loader
func (l *Loader) AddValidator(validator ConfigValidator) {
	l.validators = append(l.validators, validator)
}

// Validator implements basic ConfigValidator interface
type Validator struct {
	rules map[string][]*ValidationRule
}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{
		rules: make(map[string][]*ValidationRule),
	}
}

// ValidateStruct validates an entire configuration struct
func (v *Validator) ValidateStruct(ctx context.Context, config interface{}) error {
	// TODO: Implement struct validation
	return ErrStructValidateNotImplemented
}

// ValidateField validates a specific field with the given value
func (v *Validator) ValidateField(ctx context.Context, fieldPath string, value interface{}) error {
	// TODO: Implement field validation
	return ErrFieldValidateNotImplemented
}

// GetValidationRules returns validation rules for the given configuration type
func (v *Validator) GetValidationRules(ctx context.Context, configType string) ([]*ValidationRule, error) {
	rules, exists := v.rules[configType]
	if !exists {
		return nil, ErrConfigTypeNotFound
	}
	return rules, nil
}

// AddRule adds a validation rule for a specific configuration type
func (v *Validator) AddRule(configType string, rule *ValidationRule) {
	if v.rules[configType] == nil {
		v.rules[configType] = make([]*ValidationRule, 0)
	}
	v.rules[configType] = append(v.rules[configType], rule)
}

// Reloader implements basic ConfigReloader interface
type Reloader struct {
	watching  bool
	callbacks []ReloadCallback
}

// NewReloader creates a new configuration reloader
func NewReloader() *Reloader {
	return &Reloader{
		watching:  false,
		callbacks: make([]ReloadCallback, 0),
	}
}

// StartWatch starts watching configuration sources for changes
func (r *Reloader) StartWatch(ctx context.Context, callback ReloadCallback) error {
	// TODO: Implement configuration watching
	r.callbacks = append(r.callbacks, callback)
	r.watching = true
	return ErrStartWatchNotImplemented
}

// StopWatch stops watching configuration sources
func (r *Reloader) StopWatch(ctx context.Context) error {
	// TODO: Implement stopping configuration watch
	r.watching = false
	return ErrStopWatchNotImplemented
}

// IsWatching returns true if currently watching for configuration changes
func (r *Reloader) IsWatching() bool {
	return r.watching
}

// Helper methods for the Loader

// applyDefaults applies default values to configuration struct using reflection
func (l *Loader) applyDefaults(config interface{}) error {
	return applyDefaultsRecursive(config, "")
}

// validateRequiredFields validates that all required fields are set
func (l *Loader) validateRequiredFields(config interface{}) error {
	return validateRequiredRecursive(config, "")
}

// applyDefaultsRecursive recursively applies defaults to struct fields
func applyDefaultsRecursive(v interface{}, fieldPath string) error {
	if v == nil {
		return nil
	}

	// Use reflection to inspect the struct
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	
	if rv.Kind() != reflect.Struct {
		return nil // Only process structs
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)
		
		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Build field path
		currentPath := fieldPath
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += fieldType.Name

		// Check for default tag
		defaultValue := fieldType.Tag.Get("default")
		if defaultValue != "" && field.IsZero() {
			err := setFieldValue(field, defaultValue)
			if err != nil {
				return err
			}
		}

		// Recursively process nested structs
		if field.Kind() == reflect.Struct {
			err := applyDefaultsRecursive(field.Addr().Interface(), currentPath)
			if err != nil {
				return err
			}
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if !field.IsNil() {
				err := applyDefaultsRecursive(field.Interface(), currentPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateRequiredRecursive recursively validates required fields
func validateRequiredRecursive(v interface{}, fieldPath string) error {
	if v == nil {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	
	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		// Build field path
		currentPath := fieldPath
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += fieldType.Name

		// Check for required tag
		requiredTag := fieldType.Tag.Get("required")
		if requiredTag == "true" && field.IsZero() {
			return errors.New("required field " + currentPath + " is not set")
		}

		// Recursively process nested structs
		if field.Kind() == reflect.Struct {
			err := validateRequiredRecursive(field.Addr().Interface(), currentPath)
			if err != nil {
				return err
			}
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if !field.IsNil() {
				err := validateRequiredRecursive(field.Interface(), currentPath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// setFieldValue sets a field value from a string default using reflection
func setFieldValue(field reflect.Value, defaultValue string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(defaultValue, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(defaultValue, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(defaultValue, 64)
		if err != nil {
			return err
		}
		field.SetFloat(val)
	case reflect.Bool:
		val, err := strconv.ParseBool(defaultValue)
		if err != nil {
			return err
		}
		field.SetBool(val)
	default:
		return errors.New("unsupported field type for default value: " + field.Kind().String())
	}
	return nil
}
