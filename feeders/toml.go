package feeders

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

// TomlFeeder is a feeder that reads TOML files with optional verbose debug logging
type TomlFeeder struct {
	Path         string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewTomlFeeder creates a new TomlFeeder that reads from the specified TOML file
func NewTomlFeeder(filePath string) *TomlFeeder {
	return &TomlFeeder{
		Path:         filePath,
		verboseDebug: false,
		logger:       nil,
		fieldTracker: nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (t *TomlFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	t.verboseDebug = enabled
	t.logger = logger
	if enabled && logger != nil {
		t.logger.Debug("Verbose TOML feeder debugging enabled")
	}
}

// SetFieldTracker sets the field tracker for recording field populations
func (t *TomlFeeder) SetFieldTracker(tracker FieldTracker) {
	t.fieldTracker = tracker
}

// Feed reads the TOML file and populates the provided structure
func (t *TomlFeeder) Feed(structure interface{}) error {
	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Starting feed process", "filePath", t.Path, "structureType", reflect.TypeOf(structure))
	}

	// Always use custom parsing logic for consistency
	err := t.feedWithTracking(structure)

	if t.verboseDebug && t.logger != nil {
		if err != nil {
			t.logger.Debug("TomlFeeder: Feed completed with error", "filePath", t.Path, "error", err)
		} else {
			t.logger.Debug("TomlFeeder: Feed completed successfully", "filePath", t.Path)
		}
	}
	if err != nil {
		return fmt.Errorf("toml feed error: %w", err)
	}
	return nil
}

// FeedKey reads a TOML file and extracts a specific key
func (t *TomlFeeder) FeedKey(key string, target interface{}) error {
	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Starting FeedKey process", "filePath", t.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	err := feedKey(t, key, target, toml.Marshal, toml.Unmarshal, "TOML file")

	if t.verboseDebug && t.logger != nil {
		if err != nil {
			t.logger.Debug("TomlFeeder: FeedKey completed with error", "filePath", t.Path, "key", key, "error", err)
		} else {
			t.logger.Debug("TomlFeeder: FeedKey completed successfully", "filePath", t.Path, "key", key)
		}
	}
	return err
}

// feedWithTracking reads the TOML file and populates the provided structure with field tracking
func (t *TomlFeeder) feedWithTracking(structure interface{}) error {
	// Read and parse the TOML file manually for consistent behavior
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return fmt.Errorf("failed to read TOML file %s: %w", t.Path, err)
	}

	var tomlData map[string]interface{}
	if err := toml.Unmarshal(data, &tomlData); err != nil {
		return fmt.Errorf("failed to parse TOML file %s: %w", t.Path, err)
	}

	// Check if we're dealing with a struct pointer
	structValue := reflect.ValueOf(structure)
	if structValue.Kind() != reflect.Ptr || structValue.Elem().Kind() != reflect.Struct {
		// Not a struct pointer, fall back to standard TOML unmarshaling
		if t.verboseDebug && t.logger != nil {
			t.logger.Debug("TomlFeeder: Not a struct pointer, using standard TOML unmarshaling", "structureType", reflect.TypeOf(structure))
		}
		if err := toml.Unmarshal(data, structure); err != nil {
			return fmt.Errorf("failed to unmarshal TOML data: %w", err)
		}
		return nil
	}

	// Process the structure with field tracking
	return t.processStructFields(reflect.ValueOf(structure).Elem(), tomlData, "")
}

// processStructFields iterates through struct fields and populates them from TOML data
func (t *TomlFeeder) processStructFields(rv reflect.Value, tomlData map[string]interface{}, fieldPrefix string) error {
	structType := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get TOML tag or use field name
		tomlTag := fieldType.Tag.Get("toml")
		var tomlKey string

		if tomlTag == "" {
			// No TOML tag, use field name
			tomlKey = fieldType.Name
		} else if tomlTag == "-" {
			// Explicitly skipped
			continue
		} else {
			// Handle toml tag with options (e.g., "field,omitempty")
			tomlKey = strings.Split(tomlTag, ",")[0]
			if tomlKey == "" {
				tomlKey = fieldType.Name
			}
		}

		fieldPath := fieldType.Name // Use struct field name for path
		if fieldPrefix != "" {
			fieldPath = fieldPrefix + "." + fieldType.Name
		}

		// Check if this key exists in the TOML data
		if value, exists := tomlData[tomlKey]; exists {
			if err := t.processField(field, fieldType, value, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// processField processes a single field, handling nested structs, slices, and basic types
func (t *TomlFeeder) processField(field reflect.Value, fieldType reflect.StructField, value interface{}, fieldPath string) error {
	fieldKind := field.Kind()

	switch fieldKind {
	case reflect.Struct:
		// Handle nested structs
		if nestedMap, ok := value.(map[string]interface{}); ok {
			return t.processStructFields(field, nestedMap, fieldPath)
		}
		return wrapTomlMapError(fieldPath, value)

	case reflect.Slice:
		// Handle slices
		return t.setSliceFromTOML(field, value, fieldPath)

	case reflect.Map:
		// Handle maps
		if mapData, ok := value.(map[string]interface{}); ok {
			return t.setMapFromTOML(field, mapData, fieldType.Name, fieldPath)
		}
		return wrapTomlMapError(fieldPath, value)

	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.String,
		reflect.UnsafePointer:
		// Handle basic types and unsupported types
		return t.setFieldFromTOML(field, value, fieldPath)

	default:
		// Handle any remaining types
		return t.setFieldFromTOML(field, value, fieldPath)
	}
}

// setFieldFromTOML sets a field value from TOML data with type conversion
func (t *TomlFeeder) setFieldFromTOML(field reflect.Value, value interface{}, fieldPath string) error {
	// Convert and set the value
	convertedValue := reflect.ValueOf(value)
	if convertedValue.Type().ConvertibleTo(field.Type()) {
		field.Set(convertedValue.Convert(field.Type()))

		// Record field population
		if t.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:  fieldPath,
				FieldName:  fieldPath,
				FieldType:  field.Type().String(),
				FeederType: "TomlFeeder",
				SourceType: "toml_file",
				SourceKey:  fieldPath,
				Value:      value,
				SearchKeys: []string{fieldPath},
				FoundKey:   fieldPath,
			}
			t.fieldTracker.RecordFieldPopulation(fp)
		}

		return nil
	}

	return wrapTomlConvertError(value, field.Type().String(), fieldPath)
}

// setSliceFromTOML sets a slice field from TOML array data
func (t *TomlFeeder) setSliceFromTOML(field reflect.Value, value interface{}, fieldPath string) error {
	// Handle slice values
	if arrayValue, ok := value.([]interface{}); ok {
		sliceType := field.Type()
		elemType := sliceType.Elem()

		newSlice := reflect.MakeSlice(sliceType, len(arrayValue), len(arrayValue))

		for i, item := range arrayValue {
			elem := newSlice.Index(i)
			convertedItem := reflect.ValueOf(item)

			if convertedItem.Type().ConvertibleTo(elemType) {
				elem.Set(convertedItem.Convert(elemType))
			} else {
				return wrapTomlSliceElementError(item, elemType.String(), fieldPath, i)
			}
		}

		field.Set(newSlice)

		// Record field population for the slice
		if t.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:  fieldPath,
				FieldName:  fieldPath,
				FieldType:  field.Type().String(),
				FeederType: "TomlFeeder",
				SourceType: "toml_file",
				SourceKey:  fieldPath,
				Value:      value,
				SearchKeys: []string{fieldPath},
				FoundKey:   fieldPath,
			}
			t.fieldTracker.RecordFieldPopulation(fp)
		}

		return nil
	}

	return wrapTomlArrayError(fieldPath, value)
}

// setMapFromTOML sets a map field value from TOML data with support for pointer and value types
func (t *TomlFeeder) setMapFromTOML(field reflect.Value, tomlData map[string]interface{}, fieldName, fieldPath string) error {
	if !field.CanSet() {
		return wrapTomlFieldCannotBeSet(fieldPath)
	}

	mapType := field.Type()
	keyType := mapType.Key()
	valueType := mapType.Elem()

	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Setting map from TOML", "fieldName", fieldName, "mapType", mapType, "keyType", keyType, "valueType", valueType)
	}

	// Create a new map
	newMap := reflect.MakeMap(mapType)

	// Handle different value types
	switch valueType.Kind() {
	case reflect.Struct:
		// Map of structs, like map[string]DBConnection
		for key, value := range tomlData {
			if valueMap, ok := value.(map[string]interface{}); ok {
				// Create a new instance of the struct type
				structValue := reflect.New(valueType).Elem()

				// Process the struct fields
				if err := t.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
					return fmt.Errorf("error processing map entry '%s': %w", key, err)
				}

				// Set the map entry
				keyValue := reflect.ValueOf(key)
				newMap.SetMapIndex(keyValue, structValue)
			} else {
				if t.verboseDebug && t.logger != nil {
					t.logger.Debug("TomlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
				}
			}
		}
	case reflect.Ptr:
		// Map of pointers to structs, like map[string]*DBConnection
		elemType := valueType.Elem()
		if elemType.Kind() == reflect.Struct {
			for key, value := range tomlData {
				if value == nil {
					// Handle null values (though TOML doesn't have null, this is for consistency)
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, reflect.Zero(valueType)) // Set to nil pointer
					if t.verboseDebug && t.logger != nil {
						t.logger.Debug("TomlFeeder: Set nil pointer for null value", "key", key)
					}
				} else if valueMap, ok := value.(map[string]interface{}); ok {
					// Create a new instance of the struct type
					structValue := reflect.New(elemType).Elem()

					// Process the struct fields
					if err := t.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
						return fmt.Errorf("error processing map entry '%s': %w", key, err)
					}

					// Create a pointer to the struct and set the map entry
					ptrValue := reflect.New(elemType)
					ptrValue.Elem().Set(structValue)
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, ptrValue)

					if t.verboseDebug && t.logger != nil {
						t.logger.Debug("TomlFeeder: Successfully processed pointer to struct map entry", "key", key, "structType", elemType)
					}
				} else {
					if t.verboseDebug && t.logger != nil {
						t.logger.Debug("TomlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
					}
				}
			}
		} else {
			// Map of pointers to non-struct types - handle direct conversion
			for key, value := range tomlData {
				if value == nil {
					// Handle null values
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, reflect.Zero(valueType)) // Set to nil pointer
				} else {
					keyValue := reflect.ValueOf(key)
					valueReflect := reflect.ValueOf(value)

					// Create a new pointer to the element type
					ptrValue := reflect.New(elemType)

					if valueReflect.Type().ConvertibleTo(elemType) {
						convertedValue := valueReflect.Convert(elemType)
						ptrValue.Elem().Set(convertedValue)
						newMap.SetMapIndex(keyValue, ptrValue)
					} else {
						if t.verboseDebug && t.logger != nil {
							t.logger.Debug("TomlFeeder: Cannot convert map value for pointer type", "key", key, "valueType", valueReflect.Type(), "targetType", elemType)
						}
					}
				}
			}
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.String,
		reflect.UnsafePointer:
		// Map of primitive types - use direct conversion
		for key, value := range tomlData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				if t.verboseDebug && t.logger != nil {
					t.logger.Debug("TomlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
			}
		}
	default:
		// Map of primitive types - use direct conversion
		for key, value := range tomlData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				if t.verboseDebug && t.logger != nil {
					t.logger.Debug("TomlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
			}
		}
	}

	// Set the field to the new map
	field.Set(newMap)

	// Record field population if tracker is available
	if t.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "TomlFeeder",
			SourceType:  "toml_file",
			SourceKey:   fieldName, // For maps, use the field name as the source key
			Value:       field.Interface(),
			InstanceKey: "",
			SearchKeys:  []string{fieldName},
			FoundKey:    fieldName,
		}
		t.fieldTracker.RecordFieldPopulation(fp)
	}

	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Successfully set map field", "fieldName", fieldName, "mapSize", newMap.Len())
	}

	return nil
}
