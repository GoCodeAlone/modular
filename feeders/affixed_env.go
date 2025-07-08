// Package feeders provides configuration feeders for reading data from various sources
// including environment variables, JSON, YAML, TOML files, and .env files.
package feeders

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/golobby/cast"
)

// ErrEnvInvalidStructure indicates that the provided structure is not valid for environment variable processing
var ErrEnvInvalidStructure = errors.New("env: invalid structure")

// ErrFieldCannotBeSet indicates that a field cannot be set
var ErrFieldCannotBeSet = errors.New("field cannot be set")

// ErrEnvEmptyPrefixAndSuffix indicates that both prefix and suffix cannot be empty
var ErrEnvEmptyPrefixAndSuffix = errors.New("env: prefix or suffix cannot be empty")

// AffixedEnvFeeder is a feeder that reads environment variables with a prefix and/or suffix
type AffixedEnvFeeder struct {
	Prefix       string
	Suffix       string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewAffixedEnvFeeder creates a new AffixedEnvFeeder with the specified prefix and suffix
func NewAffixedEnvFeeder(prefix, suffix string) AffixedEnvFeeder {
	return AffixedEnvFeeder{
		Prefix:       prefix,
		Suffix:       suffix,
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *AffixedEnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	if enabled && logger != nil {
		f.logger.Debug("Verbose affixed environment feeder debugging enabled")
	}
}

// Feed reads environment variables and populates the provided structure
func (f AffixedEnvFeeder) Feed(structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Starting feed process", "structureType", reflect.TypeOf(structure), "prefix", f.Prefix, "suffix", f.Suffix)
	}

	inputType := reflect.TypeOf(structure)
	if inputType != nil {
		if inputType.Kind() == reflect.Ptr {
			if inputType.Elem().Kind() == reflect.Struct {
				return f.fillStruct(reflect.ValueOf(structure).Elem(), f.Prefix, f.Suffix)
			}
		}
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Invalid structure provided")
	}
	return ErrEnvInvalidStructure
}

// fillStruct sets struct fields from environment variables
func (f AffixedEnvFeeder) fillStruct(rv reflect.Value, prefix, suffix string) error {
	if prefix == "" && suffix == "" {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Both prefix and suffix are empty")
		}
		return ErrEnvEmptyPrefixAndSuffix
	}

	prefix = strings.ToUpper(prefix)
	suffix = strings.ToUpper(suffix)

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Processing struct with affixes", "prefix", prefix, "suffix", suffix, "structType", rv.Type())
	}

	return f.processStructFields(rv, prefix, suffix)
}

// processStructFields iterates through struct fields
func (f AffixedEnvFeeder) processStructFields(rv reflect.Value, prefix, suffix string) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Processing struct fields", "numFields", rv.NumField(), "prefix", prefix, "suffix", suffix)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rv.Type().Field(i)

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldKind", field.Kind())
		}

		if err := f.processField(field, &fieldType, prefix, suffix); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("AffixedEnvFeeder: Field processing failed", "fieldName", fieldType.Name, "error", err)
			}
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Field processing completed", "fieldName", fieldType.Name)
		}
	}
	return nil
}

// processField handles a single struct field
func (f AffixedEnvFeeder) processField(field reflect.Value, fieldType *reflect.StructField, prefix, suffix string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Processing nested struct", "fieldName", fieldType.Name, "structType", field.Type())
		}
		return f.processStructFields(field, prefix, suffix)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("AffixedEnvFeeder: Processing nested struct pointer", "fieldName", fieldType.Name, "structType", field.Elem().Type())
			}
			return f.processStructFields(field.Elem(), prefix, suffix)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("AffixedEnvFeeder: Found env tag", "fieldName", fieldType.Name, "envTag", envTag)
			}
			return f.setFieldFromEnv(field, envTag, prefix, suffix)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: No env tag found", "fieldName", fieldType.Name)
		}
	}

	return nil
}

// setFieldFromEnv sets a field value from an environment variable
func (f AffixedEnvFeeder) setFieldFromEnv(field reflect.Value, envTag, prefix, suffix string) error {
	// Build environment variable name
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = prefix + "_" + envName
	}
	if suffix != "" {
		envName = envName + "_" + suffix
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Looking up environment variable", "envName", envName, "envTag", envTag, "prefix", prefix, "suffix", suffix)
	}

	// Get and apply environment variable if exists
	if envValue := os.Getenv(envName); envValue != "" {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Environment variable found", "envName", envName, "envValue", envValue)
		}
		err := setFieldValue(field, envValue)
		if err != nil && f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Failed to set field value", "envName", envName, "envValue", envValue, "error", err)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("AffixedEnvFeeder: Successfully set field value", "envName", envName, "envValue", envValue)
		}
		return err
	} else if f.verboseDebug && f.logger != nil {
		f.logger.Debug("AffixedEnvFeeder: Environment variable not found or empty", "envName", envName)
	}
	return nil
}

// setFieldValue converts and sets a field value
func setFieldValue(field reflect.Value, strValue string) error {
	convertedValue, err := cast.FromType(strValue, field.Type())
	if err != nil {
		return fmt.Errorf("cannot convert value to type %v: %w", field.Type(), err)
	}

	if !field.CanSet() {
		return fmt.Errorf("%w of type %v", ErrFieldCannotBeSet, field.Type())
	}

	field.Set(reflect.ValueOf(convertedValue))
	return nil
}
