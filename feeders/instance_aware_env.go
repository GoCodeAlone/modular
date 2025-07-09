package feeders

import (
	"fmt"
	"reflect"
	"strings"
)

// Static errors for err113 compliance
var (
	ErrInstancesMustBeMap = fmt.Errorf("instances must be a map")
)

// InstancePrefixFunc is a function that generates a prefix for an instance key
type InstancePrefixFunc func(instanceKey string) string

// InstanceAwareEnvFeeder is a feeder that can handle environment variables for multiple instances
// of the same configuration type using instance-specific prefixes with field tracking support
type InstanceAwareEnvFeeder struct {
	prefixFunc   InstancePrefixFunc
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// Ensure InstanceAwareEnvFeeder implements all required interfaces
var _ interface {
	Feed(interface{}) error
	FeedKey(string, interface{}) error
	FeedInstances(interface{}) error
	SetVerboseDebug(bool, interface{ Debug(msg string, args ...any) })
} = (*InstanceAwareEnvFeeder)(nil)

// NewInstanceAwareEnvFeeder creates a new instance-aware environment variable feeder
func NewInstanceAwareEnvFeeder(prefixFunc InstancePrefixFunc) *InstanceAwareEnvFeeder {
	return &InstanceAwareEnvFeeder{
		prefixFunc:   prefixFunc,
		verboseDebug: false,
		logger:       nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *InstanceAwareEnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	if enabled && logger != nil {
		f.logger.Debug("Verbose instance-aware environment feeder debugging enabled")
	}
}

// SetFieldTracker sets the field tracker for this feeder
func (f *InstanceAwareEnvFeeder) SetFieldTracker(tracker FieldTracker) {
	f.fieldTracker = tracker
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Field tracker set", "hasTracker", tracker != nil)
	}
}

// Feed implements the basic Feeder interface for single instances (backward compatibility)
func (f *InstanceAwareEnvFeeder) Feed(structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Starting feed process (single instance)", "structureType", reflect.TypeOf(structure))
	}

	inputType := reflect.TypeOf(structure)
	if inputType == nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure type is nil")
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Kind() != reflect.Ptr {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure is not a pointer", "kind", inputType.Kind())
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Elem().Kind() != reflect.Struct {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure element is not a struct", "elemKind", inputType.Elem().Kind())
		}
		return ErrEnvInvalidStructure
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Feeding single instance with no prefix")
	}

	// For single instance, use no prefix
	err := f.feedStructWithPrefix(reflect.ValueOf(structure).Elem(), "", "")

	if f.verboseDebug && f.logger != nil {
		if err != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Single instance feed completed with error", "error", err)
		} else {
			f.logger.Debug("InstanceAwareEnvFeeder: Single instance feed completed successfully")
		}
	}

	return err
}

// FeedKey implements the ComplexFeeder interface for instance-specific feeding
func (f *InstanceAwareEnvFeeder) FeedKey(instanceKey string, structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Starting FeedKey process", "instanceKey", instanceKey, "structureType", reflect.TypeOf(structure))
	}

	inputType := reflect.TypeOf(structure)
	if inputType == nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure type is nil", "instanceKey", instanceKey)
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Kind() != reflect.Ptr {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure is not a pointer", "instanceKey", instanceKey, "kind", inputType.Kind())
		}
		return ErrEnvInvalidStructure
	}

	if inputType.Elem().Kind() != reflect.Struct {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Structure element is not a struct", "instanceKey", instanceKey, "elemKind", inputType.Elem().Kind())
		}
		return ErrEnvInvalidStructure
	}

	// Generate prefix for this instance
	prefix := ""
	if f.prefixFunc != nil {
		prefix = f.prefixFunc(instanceKey)
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Generated prefix for instance", "instanceKey", instanceKey, "prefix", prefix)
		}
	} else if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: No prefix function configured, using empty prefix", "instanceKey", instanceKey)
	}

	err := f.feedStructWithPrefix(reflect.ValueOf(structure).Elem(), prefix, instanceKey)

	if f.verboseDebug && f.logger != nil {
		if err != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: FeedKey completed with error", "instanceKey", instanceKey, "prefix", prefix, "error", err)
		} else {
			f.logger.Debug("InstanceAwareEnvFeeder: FeedKey completed successfully", "instanceKey", instanceKey, "prefix", prefix)
		}
	}

	return err
}

// FeedInstances feeds multiple instances of the same configuration type
func (f *InstanceAwareEnvFeeder) FeedInstances(instances interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Starting FeedInstances process", "instancesType", reflect.TypeOf(instances))
	}

	instancesValue := reflect.ValueOf(instances)
	if instancesValue.Kind() != reflect.Map {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Instances is not a map", "kind", instancesValue.Kind())
		}
		return ErrInstancesMustBeMap
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Processing map instances", "instanceCount", instancesValue.Len())
	}

	// Iterate through map entries
	for _, key := range instancesValue.MapKeys() {
		instanceKey := key.String()
		instance := instancesValue.MapIndex(key)

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Processing instance", "instanceKey", instanceKey, "instanceType", instance.Type())
		}

		// Create a pointer to the instance for modification
		instancePtr := reflect.New(instance.Type())
		instancePtr.Elem().Set(instance)

		// Feed this instance with its specific prefix
		if err := f.FeedKey(instanceKey, instancePtr.Interface()); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("InstanceAwareEnvFeeder: Failed to feed instance", "instanceKey", instanceKey, "error", err)
			}
			return fmt.Errorf("failed to feed instance '%s': %w", instanceKey, err)
		}

		// Update the map with the modified instance
		instancesValue.SetMapIndex(key, instancePtr.Elem())

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Successfully fed instance", "instanceKey", instanceKey)
		}
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: FeedInstances completed successfully")
	}

	return nil
}

// feedStructWithPrefix feeds a struct with environment variables using the specified prefix
func (f *InstanceAwareEnvFeeder) feedStructWithPrefix(rv reflect.Value, prefix, instanceKey string) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Starting feedStructWithPrefix", "structType", rv.Type(), "prefix", prefix, "instanceKey", instanceKey)
	}
	return f.processStructFieldsWithPrefix(rv, prefix, "", instanceKey)
}

// processStructFieldsWithPrefix iterates through struct fields with prefix
func (f *InstanceAwareEnvFeeder) processStructFieldsWithPrefix(rv reflect.Value, prefix, parentPath, instanceKey string) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Processing struct fields", "structType", rv.Type(), "numFields", rv.NumField(), "prefix", prefix, "parentPath", parentPath, "instanceKey", instanceKey)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rv.Type().Field(i)

		// Build field path
		fieldPath := fieldType.Name
		if parentPath != "" {
			fieldPath = parentPath + "." + fieldType.Name
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldKind", field.Kind(), "prefix", prefix, "fieldPath", fieldPath, "instanceKey", instanceKey)
		}

		if err := f.processFieldWithPrefix(field, &fieldType, prefix, fieldPath, instanceKey); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("InstanceAwareEnvFeeder: Field processing failed", "fieldName", fieldType.Name, "prefix", prefix, "error", err)
			}
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Field processing completed", "fieldName", fieldType.Name, "prefix", prefix)
		}
	}
	return nil
}

// processFieldWithPrefix handles a single struct field with prefix
func (f *InstanceAwareEnvFeeder) processFieldWithPrefix(field reflect.Value, fieldType *reflect.StructField, prefix, fieldPath, instanceKey string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Processing nested struct", "fieldName", fieldType.Name, "structType", field.Type(), "prefix", prefix, "fieldPath", fieldPath, "instanceKey", instanceKey)
		}
		return f.processStructFieldsWithPrefix(field, prefix, fieldPath, instanceKey)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("InstanceAwareEnvFeeder: Processing nested struct pointer", "fieldName", fieldType.Name, "structType", field.Elem().Type(), "prefix", prefix, "fieldPath", fieldPath, "instanceKey", instanceKey)
			}
			return f.processStructFieldsWithPrefix(field.Elem(), prefix, fieldPath, instanceKey)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("InstanceAwareEnvFeeder: Found env tag", "fieldName", fieldType.Name, "envTag", envTag, "prefix", prefix, "fieldPath", fieldPath, "instanceKey", instanceKey)
			}
			return f.setFieldFromEnvWithPrefix(field, envTag, prefix, fieldType.Name, fieldPath, instanceKey)
		} else if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: No env tag found", "fieldName", fieldType.Name, "prefix", prefix, "fieldPath", fieldPath, "instanceKey", instanceKey)
		}
	}

	return nil
}

// setFieldFromEnvWithPrefix sets a field value from an environment variable with prefix and field tracking
func (f *InstanceAwareEnvFeeder) setFieldFromEnvWithPrefix(field reflect.Value, envTag, prefix, fieldName, fieldPath, instanceKey string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Track what we're searching for
	searchKeys := []string{envName}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("InstanceAwareEnvFeeder: Looking up environment variable", "envName", envName, "envTag", envTag, "prefix", prefix, "fieldName", fieldName, "fieldPath", fieldPath, "instanceKey", instanceKey)
	}

	// Get and apply environment variable if exists
	catalog := GetGlobalEnvCatalog()
	envValue, exists := catalog.Get(envName)
	if exists && envValue != "" {
		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(envName)
			f.logger.Debug("InstanceAwareEnvFeeder: Environment variable found", "envName", envName, "envValue", envValue, "fieldPath", fieldPath, "instanceKey", instanceKey, "source", source)
		}

		err := setFieldValue(field, envValue)
		if err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("InstanceAwareEnvFeeder: Failed to set field value", "envName", envName, "envValue", envValue, "error", err, "fieldPath", fieldPath, "instanceKey", instanceKey)
			}
			return err
		}

		// Record field population if tracker is available
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.InstanceAwareEnvFeeder",
				SourceType:  "env",
				SourceKey:   envName,
				Value:       field.Interface(),
				InstanceKey: instanceKey,
				SearchKeys:  searchKeys,
				FoundKey:    envName,
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Successfully set field value", "envName", envName, "envValue", envValue, "fieldPath", fieldPath, "instanceKey", instanceKey)
		}
	} else {
		// Record that we searched but didn't find
		if f.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.InstanceAwareEnvFeeder",
				SourceType:  "env",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: instanceKey,
				SearchKeys:  searchKeys,
				FoundKey:    "",
			}
			f.fieldTracker.RecordFieldPopulation(fp)
		}

		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("InstanceAwareEnvFeeder: Environment variable not found or empty", "envName", envName, "fieldPath", fieldPath, "instanceKey", instanceKey)
		}
	}

	return nil
}
