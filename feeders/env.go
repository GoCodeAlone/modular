package feeders

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// EnvFeeder is a feeder that reads environment variables with optional verbose debug logging
type EnvFeeder struct {
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
}

// NewEnvFeeder creates a new EnvFeeder that reads from environment variables
func NewEnvFeeder() EnvFeeder {
	return EnvFeeder{
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *EnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	if enabled && logger != nil {
		f.logger.Debug("Verbose environment feeder debugging enabled")
	}
}

// Feed implements the Feeder interface with optional verbose logging
func (f EnvFeeder) Feed(structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Starting feed process", "structureType", reflect.TypeOf(structure))
	}

	inputType := reflect.TypeOf(structure)
	if inputType == nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Structure type is nil")
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Kind() != reflect.Ptr {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Structure is not a pointer", "kind", inputType.Kind())
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Elem().Kind() != reflect.Struct {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Structure element is not a struct", "elemKind", inputType.Elem().Kind())
		}
		return ErrEnvInvalidStructure
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Processing struct fields", "structType", inputType.Elem())
	}

	err := f.processStructFields(reflect.ValueOf(structure).Elem(), "")

	if f.verboseDebug && f.logger != nil {
		if err != nil {
			f.logger.Debug("EnvFeeder: Feed completed with error", "error", err)
		} else {
			f.logger.Debug("EnvFeeder: Feed completed successfully")
		}
	}

	return err
}

// processStructFields processes all fields in a struct with optional verbose logging
func (f EnvFeeder) processStructFields(rv reflect.Value, prefix string) error {
	structType := rv.Type()

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Processing struct", "structType", structType, "numFields", rv.NumField(), "prefix", prefix)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldKind", field.Kind())
		}

		if err := f.processField(field, &fieldType, prefix); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Field processing failed", "fieldName", fieldType.Name, "error", err)
			}
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Field processing completed", "fieldName", fieldType.Name)
		}
	}
	return nil
}

// processField handles a single struct field with optional verbose logging
func (f EnvFeeder) processField(field reflect.Value, fieldType *reflect.StructField, prefix string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Processing nested struct", "fieldName", fieldType.Name, "structType", field.Type())
		}
		return f.processStructFields(field, prefix)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Processing nested struct pointer", "fieldName", fieldType.Name, "structType", field.Elem().Type())
			}
			return f.processStructFields(field.Elem(), prefix)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Found env tag", "fieldName", fieldType.Name, "envTag", envTag)
			}
			return f.setFieldFromEnv(field, envTag, prefix, fieldType.Name)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: No env tag found", "fieldName", fieldType.Name)
		}
	}

	return nil
}

// setFieldFromEnv sets a field value from an environment variable with optional verbose logging
func (f EnvFeeder) setFieldFromEnv(field reflect.Value, envTag, prefix, fieldName string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Looking up environment variable", "fieldName", fieldName, "envName", envName, "envTag", envTag, "prefix", prefix)
	}

	// Get and apply environment variable if exists
	envValue := os.Getenv(envName)
	if envValue != "" {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Environment variable found", "fieldName", fieldName, "envName", envName, "envValue", envValue)
		}

		err := setFieldValue(field, envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Failed to set field value", "fieldName", fieldName, "envName", envName, "envValue", envValue, "error", err)
			}
			return err
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Successfully set field value", "fieldName", fieldName, "envName", envName, "envValue", envValue)
		}
	} else {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Environment variable not found or empty", "fieldName", fieldName, "envName", envName)
		}
	}

	return nil
}
