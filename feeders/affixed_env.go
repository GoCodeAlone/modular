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

// ErrEnvEmptyPrefixAndSuffix indicates that both prefix and suffix cannot be empty
var ErrEnvEmptyPrefixAndSuffix = errors.New("env: prefix or suffix cannot be empty")

// AffixedEnvFeeder is a feeder that reads environment variables with a prefix and/or suffix
type AffixedEnvFeeder struct {
	Prefix string
	Suffix string
}

// NewAffixedEnvFeeder creates a new AffixedEnvFeeder with the specified prefix and suffix
func NewAffixedEnvFeeder(prefix, suffix string) AffixedEnvFeeder {
	return AffixedEnvFeeder{Prefix: prefix, Suffix: suffix}
}

// Feed reads environment variables and populates the provided structure
func (f AffixedEnvFeeder) Feed(structure interface{}) error {
	inputType := reflect.TypeOf(structure)
	if inputType != nil {
		if inputType.Kind() == reflect.Ptr {
			if inputType.Elem().Kind() == reflect.Struct {
				return fillStruct(reflect.ValueOf(structure).Elem(), f.Prefix, f.Suffix)
			}
		}
	}

	return ErrEnvInvalidStructure
}

// fillStruct sets struct fields from environment variables
func fillStruct(rv reflect.Value, prefix, suffix string) error {
	if prefix == "" && suffix == "" {
		return ErrEnvEmptyPrefixAndSuffix
	}

	prefix = strings.ToUpper(prefix)
	suffix = strings.ToUpper(suffix)

	return processStructFields(rv, prefix, suffix)
}

// processStructFields iterates through struct fields
func processStructFields(rv reflect.Value, prefix, suffix string) error {
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rv.Type().Field(i)

		if err := processField(field, &fieldType, prefix, suffix); err != nil {
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}
	}
	return nil
}

// processField handles a single struct field
func processField(field reflect.Value, fieldType *reflect.StructField, prefix, suffix string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		return processStructFields(field, prefix, suffix)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			return processStructFields(field.Elem(), prefix, suffix)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			return setFieldFromEnv(field, envTag, prefix, suffix)
		}
	}

	return nil
}

// setFieldFromEnv sets a field value from an environment variable
func setFieldFromEnv(field reflect.Value, envTag, prefix, suffix string) error {
	// Build environment variable name
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = prefix + "_" + envName
	}
	if suffix != "" {
		envName = envName + "_" + suffix
	}

	// Get and apply environment variable if exists
	if envValue := os.Getenv(envName); envValue != "" {
		return setFieldValue(field, envValue)
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
		return fmt.Errorf("field cannot be set")
	}

	field.Set(reflect.ValueOf(convertedValue))
	return nil
}
