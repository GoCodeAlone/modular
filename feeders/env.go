package feeders

import (
	"fmt"
	"reflect"
	"strings"
)

// EnvFeeder is a feeder that reads environment variables with optional verbose debug logging and field tracking
type EnvFeeder struct {
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewEnvFeeder creates a new EnvFeeder that reads from environment variables
func NewEnvFeeder() *EnvFeeder {
	return &EnvFeeder{
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

// SetFieldTracker sets the field tracker for this feeder
func (f *EnvFeeder) SetFieldTracker(tracker FieldTracker) {
	f.fieldTracker = tracker
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Field tracker set", "hasTracker", tracker != nil)
	}
}

// Feed implements the Feeder interface with optional verbose logging
func (f *EnvFeeder) Feed(structure interface{}) error {
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

	err := f.processStructFields(reflect.ValueOf(structure).Elem(), "", "")

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
func (f *EnvFeeder) processStructFields(rv reflect.Value, prefix, parentPath string) error {
	structType := rv.Type()

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Processing struct", "structType", structType, "numFields", rv.NumField(), "prefix", prefix, "parentPath", parentPath)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Build field path
		fieldPath := fieldType.Name
		if parentPath != "" {
			fieldPath = parentPath + "." + fieldType.Name
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldKind", field.Kind(), "fieldPath", fieldPath)
		}

		if err := f.processField(field, &fieldType, prefix, fieldPath); err != nil {
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
func (f *EnvFeeder) processField(field reflect.Value, fieldType *reflect.StructField, prefix, fieldPath string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Processing nested struct", "fieldName", fieldType.Name, "structType", field.Type(), "fieldPath", fieldPath)
		}
		return f.processStructFields(field, prefix, fieldPath)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Processing nested struct pointer", "fieldName", fieldType.Name, "structType", field.Elem().Type(), "fieldPath", fieldPath)
			}
			return f.processStructFields(field.Elem(), prefix, fieldPath)
		} else {
			// Handle pointers to primitive types or nil pointers with env tags
			if envTag, exists := fieldType.Tag.Lookup("env"); exists {
				if f.verboseDebug && f.logger != nil {
					f.logger.Debug("EnvFeeder: Found env tag for pointer field", "fieldName", fieldType.Name, "envTag", envTag, "fieldPath", fieldPath)
				}
				return f.setPointerFieldFromEnv(field, envTag, prefix, fieldType.Name, fieldPath)
			} else if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: No env tag found for pointer field", "fieldName", fieldType.Name, "fieldPath", fieldPath)
			}
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Found env tag", "fieldName", fieldType.Name, "envTag", envTag, "fieldPath", fieldPath)
			}
			return f.setFieldFromEnv(field, envTag, prefix, fieldType.Name, fieldPath)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: No env tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}
	}

	return nil
}

// setFieldFromEnv sets a field value from an environment variable with optional verbose logging and field tracking
func (f *EnvFeeder) setFieldFromEnv(field reflect.Value, envTag, prefix, fieldName, fieldPath string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Track what we're searching for
	searchKeys := []string{envName}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Looking up environment variable", "fieldName", fieldName, "envName", envName, "envTag", envTag, "prefix", prefix, "fieldPath", fieldPath)
	}

	// Get and apply environment variable if exists
	catalog := GetGlobalEnvCatalog()
	envValue, exists := catalog.Get(envName)
	if exists && envValue != "" {
		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(envName)
			f.logger.Debug("EnvFeeder: Environment variable found", "fieldName", fieldName, "envName", envName, "envValue", envValue, "fieldPath", fieldPath, "source", source)
		}

		err := setFieldValue(field, envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Failed to set field value", "fieldName", fieldName, "envName", envName, "envValue", envValue, "error", err, "fieldPath", fieldPath)
			}
			return err
		}

		// Record field population if tracker is available
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.EnvFeeder",
				SourceType:  "env",
				SourceKey:   envName,
				Value:       field.Interface(),
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    envName,
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Successfully set field value", "fieldName", fieldName, "envName", envName, "envValue", envValue, "fieldPath", fieldPath)
		}
	} else {
		// Record that we searched but didn't find
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.EnvFeeder",
				SourceType:  "env",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    "",
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Environment variable not found or empty", "fieldName", fieldName, "envName", envName, "fieldPath", fieldPath)
		}
	}

	return nil
}

// setPointerFieldFromEnv sets a pointer field value from an environment variable
func (f *EnvFeeder) setPointerFieldFromEnv(field reflect.Value, envTag, prefix, fieldName, fieldPath string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Track what we're searching for
	searchKeys := []string{envName}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Looking up environment variable for pointer field", "fieldName", fieldName, "envName", envName, "envTag", envTag, "prefix", prefix, "fieldPath", fieldPath)
	}

	// Get and apply environment variable if exists
	catalog := GetGlobalEnvCatalog()
	envValue, exists := catalog.Get(envName)
	if exists && envValue != "" {
		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(envName)
			f.logger.Debug("EnvFeeder: Environment variable found for pointer field", "fieldName", fieldName, "envName", envName, "envValue", envValue, "fieldPath", fieldPath, "source", source)
		}

		// Get the type that the pointer points to
		elemType := field.Type().Elem()

		// Create a new instance of the pointed-to type
		newValue := reflect.New(elemType)

		// Set the value using the existing setFieldValue function
		err := setFieldValue(newValue.Elem(), envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Failed to set pointer field value", "fieldName", fieldName, "envName", envName, "envValue", envValue, "error", err, "fieldPath", fieldPath)
			}
			return err
		}

		// Set the field to point to the new value
		field.Set(newValue)

		// Record field population if tracker is available
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.EnvFeeder",
				SourceType:  "env",
				SourceKey:   envName,
				Value:       field.Interface(),
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    envName,
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Successfully set pointer field", "fieldName", fieldName, "envName", envName, "fieldPath", fieldPath)
		}
	} else {
		// Record that we searched but didn't find
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.EnvFeeder",
				SourceType:  "env",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    "",
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Environment variable not found or empty for pointer field", "fieldName", fieldName, "envName", envName, "fieldPath", fieldPath)
		}
	}

	return nil
}
