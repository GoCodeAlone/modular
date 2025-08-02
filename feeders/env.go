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
	// Use the FeedWithModuleContext method with empty module name for backward compatibility
	return f.FeedWithModuleContext(structure, "")
}

// FeedWithModuleContext implements module-aware feeding that searches for module-prefixed environment variables
func (f *EnvFeeder) FeedWithModuleContext(structure interface{}, moduleName string) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Starting feed process", "structureType", reflect.TypeOf(structure), "moduleName", moduleName)
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

	err := f.processStructFieldsWithModule(reflect.ValueOf(structure).Elem(), "", "", moduleName)

	if f.verboseDebug && f.logger != nil {
		if err != nil {
			f.logger.Debug("EnvFeeder: Feed completed with error", "error", err)
		} else {
			f.logger.Debug("EnvFeeder: Feed completed successfully")
		}
	}

	return err
}

// processStructFieldsWithModule processes all fields in a struct with module awareness
func (f *EnvFeeder) processStructFieldsWithModule(rv reflect.Value, prefix, parentPath, moduleName string) error {
	structType := rv.Type()

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Processing struct", "structType", structType, "numFields", rv.NumField(), "prefix", prefix, "parentPath", parentPath, "moduleName", moduleName)
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

		if err := f.processFieldWithModule(field, &fieldType, prefix, fieldPath, moduleName); err != nil {
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

// processFieldWithModule handles a single struct field with module awareness
func (f *EnvFeeder) processFieldWithModule(field reflect.Value, fieldType *reflect.StructField, prefix, fieldPath, moduleName string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Processing nested struct", "fieldName", fieldType.Name, "structType", field.Type(), "fieldPath", fieldPath)
		}
		return f.processStructFieldsWithModule(field, prefix, fieldPath, moduleName)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Processing nested struct pointer", "fieldName", fieldType.Name, "structType", field.Elem().Type(), "fieldPath", fieldPath)
			}
			return f.processStructFieldsWithModule(field.Elem(), prefix, fieldPath, moduleName)
		} else {
			// Handle pointers to primitive types or nil pointers with env tags
			if envTag, exists := fieldType.Tag.Lookup("env"); exists {
				if f.verboseDebug && f.logger != nil {
					f.logger.Debug("EnvFeeder: Found env tag for pointer field", "fieldName", fieldType.Name, "envTag", envTag, "fieldPath", fieldPath)
				}
				return f.setPointerFieldFromEnvWithModule(field, envTag, prefix, fieldType.Name, fieldPath, moduleName)
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
			return f.setFieldFromEnvWithModule(field, envTag, prefix, fieldType.Name, fieldPath, moduleName)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: No env tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}
	}

	return nil
}

// setFieldFromEnvWithModule sets a field value from an environment variable with module-aware searching
func (f *EnvFeeder) setFieldFromEnvWithModule(field reflect.Value, envTag, prefix, fieldName, fieldPath, moduleName string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Build search keys in priority order (module-aware searching)
	searchKeys := f.buildSearchKeys(envName, moduleName)

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Looking up environment variable", "fieldName", fieldName, "envTag", envTag, "prefix", prefix, "fieldPath", fieldPath, "moduleName", moduleName, "searchKeys", searchKeys)
	}

	// Search for environment variables in priority order
	catalog := GetGlobalEnvCatalog()
	var foundKey string
	var envValue string
	var exists bool

	for _, searchKey := range searchKeys {
		envValue, exists = catalog.Get(searchKey)
		if exists && envValue != "" {
			foundKey = searchKey
			break
		}
	}

	if exists && envValue != "" {
		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(foundKey)
			f.logger.Debug("EnvFeeder: Environment variable found", "fieldName", fieldName, "foundKey", foundKey, "envValue", envValue, "fieldPath", fieldPath, "source", source)
		}

		err := setFieldValue(field, envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Failed to set field value", "fieldName", fieldName, "foundKey", foundKey, "envValue", envValue, "error", err, "fieldPath", fieldPath)
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
				SourceKey:   foundKey,
				Value:       field.Interface(),
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    foundKey,
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Successfully set field value", "fieldName", fieldName, "foundKey", foundKey, "envValue", envValue, "fieldPath", fieldPath)
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
			f.logger.Debug("EnvFeeder: Environment variable not found or empty", "fieldName", fieldName, "searchKeys", searchKeys, "fieldPath", fieldPath)
		}
	}

	return nil
}

// buildSearchKeys creates a list of environment variable names to search in priority order
// Implements the search pattern: MODULE_ENV_VAR, ENV_VAR_MODULE, ENV_VAR
func (f *EnvFeeder) buildSearchKeys(envName, moduleName string) []string {
	var searchKeys []string

	// If we have a module name, build module-aware search keys
	if moduleName != "" && strings.TrimSpace(moduleName) != "" {
		moduleUpper := strings.ToUpper(strings.TrimSpace(moduleName))

		// 1. MODULE_ENV_VAR (prefix)
		searchKeys = append(searchKeys, moduleUpper+"_"+envName)

		// 2. ENV_VAR_MODULE (suffix)
		searchKeys = append(searchKeys, envName+"_"+moduleUpper)
	}

	// 3. ENV_VAR (original behavior)
	searchKeys = append(searchKeys, envName)

	return searchKeys
}

// setPointerFieldFromEnvWithModule sets a pointer field value from an environment variable with module awareness
func (f *EnvFeeder) setPointerFieldFromEnvWithModule(field reflect.Value, envTag, prefix, fieldName, fieldPath, moduleName string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Build search keys in priority order (module-aware searching)
	searchKeys := f.buildSearchKeys(envName, moduleName)

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("EnvFeeder: Looking up environment variable for pointer field", "fieldName", fieldName, "envTag", envTag, "prefix", prefix, "fieldPath", fieldPath, "moduleName", moduleName, "searchKeys", searchKeys)
	}

	// Search for environment variables in priority order
	catalog := GetGlobalEnvCatalog()
	var foundKey string
	var envValue string
	var exists bool

	for _, searchKey := range searchKeys {
		envValue, exists = catalog.Get(searchKey)
		if exists && envValue != "" {
			foundKey = searchKey
			break
		}
	}

	if exists && envValue != "" {
		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(foundKey)
			f.logger.Debug("EnvFeeder: Environment variable found for pointer field", "fieldName", fieldName, "foundKey", foundKey, "envValue", envValue, "fieldPath", fieldPath, "source", source)
		}

		// Get the type that the pointer points to
		elemType := field.Type().Elem()

		// Create a new instance of the pointed-to type
		newValue := reflect.New(elemType)

		// Set the value using the existing setFieldValue function
		err := setFieldValue(newValue.Elem(), envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("EnvFeeder: Failed to set pointer field value", "fieldName", fieldName, "foundKey", foundKey, "envValue", envValue, "error", err, "fieldPath", fieldPath)
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
				SourceKey:   foundKey,
				Value:       field.Interface(),
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    foundKey,
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("EnvFeeder: Successfully set pointer field", "fieldName", fieldName, "foundKey", foundKey, "fieldPath", fieldPath)
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
			f.logger.Debug("EnvFeeder: Environment variable not found or empty for pointer field", "fieldName", fieldName, "searchKeys", searchKeys, "fieldPath", fieldPath)
		}
	}

	return nil
}
