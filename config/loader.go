// Package config provides configuration loading and management services
package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Static errors for configuration package
var (
	ErrLoadNotImplemented                = errors.New("load method not yet implemented")
	ErrReloadNotImplemented              = errors.New("reload method not yet implemented")
	ErrValidateNotImplemented            = errors.New("validate method not yet implemented")
	ErrProvenanceNotImplemented          = errors.New("provenance method not yet implemented")
	ErrStructValidateNotImplemented      = errors.New("struct validation not yet implemented")
	ErrFieldValidateNotImplemented       = errors.New("field validation not yet implemented")
	ErrStartWatchNotImplemented          = errors.New("start watch method not yet implemented")
	ErrStopWatchNotImplemented           = errors.New("stop watch method not yet implemented")
	ErrConfigTypeNotFound                = errors.New("config type not found")
	ErrConfigCannotBeNil                 = errors.New("config cannot be nil")
	ErrNoProvenanceInfo                  = errors.New("no provenance information found for field")
	ErrRequiredFieldNotSet               = errors.New("required field is not set")
	ErrUnsupportedFieldType              = errors.New("unsupported field type for default value")
	ErrServiceRegistrationConflict       = errors.New("service registration conflict: service name already exists")
	ErrUnknownConflictResolutionStrategy = errors.New("unknown conflict resolution strategy")
	ErrAmbiguousMultipleServices         = errors.New("ambiguous interface resolution: multiple services with equal priority and registration time")
)

// Loader implements the ConfigLoader interface with basic stub functionality
type Loader struct {
	sources    []*ConfigSource
	validators []ConfigValidator
	provenance map[string]*FieldProvenance // Track provenance by field path
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		sources:    make([]*ConfigSource, 0),
		validators: make([]ConfigValidator, 0),
		provenance: make(map[string]*FieldProvenance),
	}
}

// Load loads configuration from all configured sources and applies validation
func (l *Loader) Load(ctx context.Context, config interface{}) error {
	if config == nil {
		return ErrConfigCannotBeNil
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
	if config == nil {
		return ErrConfigCannotBeNil
	}

	// Clear previous provenance information for fresh reload
	l.provenance = make(map[string]*FieldProvenance)

	// Reload from all sources in priority order
	for _, source := range l.sources {
		err := l.loadFromSource(ctx, config, source)
		if err != nil {
			// Mark source as failed but continue with other sources
			source.Error = err.Error()
			source.Loaded = false
			continue
		}

		// Mark source as successfully loaded
		now := time.Now()
		source.LastLoaded = &now
		source.Loaded = true
		source.Error = ""
	}

	// Apply defaults for any fields not set by sources
	err := l.applyDefaults(config)
	if err != nil {
		return fmt.Errorf("failed to apply defaults during reload: %w", err)
	}

	// Re-run validation after reload
	err = l.Validate(ctx, config)
	if err != nil {
		return fmt.Errorf("validation failed during reload: %w", err)
	}

	return nil
}

// loadFromSource loads configuration from a specific source
func (l *Loader) loadFromSource(ctx context.Context, config interface{}, source *ConfigSource) error {
	// TODO: Implement actual loading from different source types
	// For now, this is a placeholder that would delegate to appropriate
	// feeders based on source.Type (env, yaml, json, toml, etc.)

	// Record provenance information for fields loaded from this source
	// This would be done by the actual feeder implementations
	l.recordProvenance("placeholder.field", source.Name, source.Location, "placeholder_value")

	return nil
}

// recordProvenance records provenance information for a configuration field
func (l *Loader) recordProvenance(fieldPath, source, sourceDetail string, value interface{}) {
	l.provenance[fieldPath] = &FieldProvenance{
		FieldPath:    fieldPath,
		Source:       source,
		SourceDetail: sourceDetail,
		Value:        value,
		Timestamp:    time.Now(),
		Metadata:     make(map[string]string),
	}
}

// Validate validates the given configuration against defined rules and schemas
func (l *Loader) Validate(ctx context.Context, config interface{}) error {
	// Validate using all configured validators
	for _, validator := range l.validators {
		err := validator.ValidateStruct(ctx, config)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
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
	// Look up provenance information for the field path
	if provenance, exists := l.provenance[fieldPath]; exists {
		return provenance, nil
	}

	// If no provenance tracked, return not found error
	return nil, fmt.Errorf("%w: %s", ErrNoProvenanceInfo, fieldPath)
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
	return l.applyDefaultsRecursive(config, "")
}

// validateRequiredFields validates that all required fields are set
func (l *Loader) validateRequiredFields(config interface{}) error {
	return validateRequiredRecursive(config, "")
}

// applyDefaultsRecursive recursively applies defaults to struct fields
func (l *Loader) applyDefaultsRecursive(v interface{}, fieldPath string) error {
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

			// Track provenance for this field
			l.provenance[currentPath] = &FieldProvenance{
				FieldPath:    currentPath,
				Source:       "default",
				SourceDetail: "struct-tag:" + fieldType.Name,
				Value:        defaultValue,
				Timestamp:    time.Now(),
				Metadata: map[string]string{
					"field_type": fieldType.Type.String(),
					"tag_value":  defaultValue,
				},
			}
		}

		// Recursively process nested structs
		if field.Kind() == reflect.Struct {
			err := l.applyDefaultsRecursive(field.Addr().Interface(), currentPath)
			if err != nil {
				return err
			}
		} else if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if !field.IsNil() {
				err := l.applyDefaultsRecursive(field.Interface(), currentPath)
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
			return fmt.Errorf("%w: %s", ErrRequiredFieldNotSet, currentPath)
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
			return fmt.Errorf("parsing int value %q: %w", defaultValue, err)
		}
		field.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(defaultValue, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing uint value %q: %w", defaultValue, err)
		}
		field.SetUint(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(defaultValue, 64)
		if err != nil {
			return fmt.Errorf("parsing float value %q: %w", defaultValue, err)
		}
		field.SetFloat(val)
	case reflect.Bool:
		val, err := strconv.ParseBool(defaultValue)
		if err != nil {
			return fmt.Errorf("parsing bool value %q: %w", defaultValue, err)
		}
		field.SetBool(val)
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Ptr, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		// These types are not supported for default values
		return fmt.Errorf("%w: %s", ErrUnsupportedFieldType, field.Kind().String())
	default:
		// Fallback for any other types
		return fmt.Errorf("%w: %s", ErrUnsupportedFieldType, field.Kind().String())
	}
	return nil
}

// RedactSecrets redacts sensitive field values in provenance information
func (l *Loader) RedactSecrets(provenance *FieldProvenance) *FieldProvenance {
	if provenance == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	redacted := &FieldProvenance{
		FieldPath:    provenance.FieldPath,
		Source:       provenance.Source,
		SourceDetail: provenance.SourceDetail,
		Value:        provenance.Value,
		Timestamp:    provenance.Timestamp,
		Metadata:     make(map[string]string),
	}

	// Copy metadata
	for k, v := range provenance.Metadata {
		redacted.Metadata[k] = v
	}

	// Check if field contains sensitive data
	if isSecretField(provenance.FieldPath) {
		redacted.Value = "[REDACTED]"
		redacted.Metadata["redacted"] = "true"
		redacted.Metadata["redaction_reason"] = "secret_field"
	}

	return redacted
}

// isSecretField determines if a field path contains sensitive information
func isSecretField(fieldPath string) bool {
	// Simple pattern matching for common secret field names
	secretPatterns := []string{
		"password", "secret", "key", "token", "credential",
		"auth", "private", "cert", "ssl", "tls",
	}

	lowerPath := strings.ToLower(fieldPath)
	for _, pattern := range secretPatterns {
		if contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (simple implementation)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
