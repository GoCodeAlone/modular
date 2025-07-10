package feeders

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// Feeder interface for common operations
type Feeder interface {
	Feed(target interface{}) error
}

// feedKey is a common helper function for extracting specific keys from config files
func feedKey(
	feeder Feeder,
	key string,
	target interface{},
	marshalFunc func(interface{}) ([]byte, error),
	unmarshalFunc func([]byte, interface{}) error,
	fileType string,
) error {
	// Create a temporary map to hold all data
	var allData map[string]interface{}

	// Use the feeder to read the file
	if err := feeder.Feed(&allData); err != nil {
		return fmt.Errorf("failed to read %s: %w", fileType, err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		return nil
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := marshalFunc(value)
	if err != nil {
		return fmt.Errorf("failed to marshal %s data: %w", fileType, err)
	}

	if err = unmarshalFunc(valueBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal %s data: %w", fileType, err)
	}

	return nil
}

// JSONFeeder is a feeder that reads JSON files with optional verbose debug logging
type JSONFeeder struct {
	Path         string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewJSONFeeder creates a new JSONFeeder that reads from the specified JSON file
func NewJSONFeeder(filePath string) *JSONFeeder {
	return &JSONFeeder{
		Path:         filePath,
		verboseDebug: false,
		logger:       nil,
		fieldTracker: nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (j *JSONFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	j.verboseDebug = enabled
	j.logger = logger
	if enabled && logger != nil {
		j.logger.Debug("Verbose JSON feeder debugging enabled")
	}
}

// Feed reads the JSON file and populates the provided structure
func (j *JSONFeeder) Feed(structure interface{}) error {
	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Starting feed process", "filePath", j.Path, "structureType", reflect.TypeOf(structure))
	}

	// Always use custom parsing logic for consistency
	err := j.feedWithTracking(structure)

	if j.verboseDebug && j.logger != nil {
		if err != nil {
			j.logger.Debug("JSONFeeder: Feed completed with error", "filePath", j.Path, "error", err)
		} else {
			j.logger.Debug("JSONFeeder: Feed completed successfully", "filePath", j.Path)
		}
	}
	if err != nil {
		return fmt.Errorf("json feed error: %w", err)
	}
	return nil
}

// FeedKey reads a JSON file and extracts a specific key
func (j *JSONFeeder) FeedKey(key string, target interface{}) error {
	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Starting FeedKey process", "filePath", j.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	err := feedKey(j, key, target, json.Marshal, json.Unmarshal, "JSON file")

	if j.verboseDebug && j.logger != nil {
		if err != nil {
			j.logger.Debug("JSONFeeder: FeedKey completed with error", "filePath", j.Path, "key", key, "error", err)
		} else {
			j.logger.Debug("JSONFeeder: FeedKey completed successfully", "filePath", j.Path, "key", key)
		}
	}
	return err
}

// SetFieldTracker sets the field tracker for recording field populations
func (j *JSONFeeder) SetFieldTracker(tracker FieldTracker) {
	j.fieldTracker = tracker
}

// feedWithTracking reads the JSON file and populates the provided structure with field tracking
func (j *JSONFeeder) feedWithTracking(structure interface{}) error {
	// Read and parse the JSON file manually for consistent behavior
	data, err := os.ReadFile(j.Path)
	if err != nil {
		return fmt.Errorf("failed to read JSON file %s: %w", j.Path, err)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to parse JSON file %s: %w", j.Path, err)
	}

	// Check if we're dealing with a struct pointer
	structValue := reflect.ValueOf(structure)
	if structValue.Kind() != reflect.Ptr || structValue.Elem().Kind() != reflect.Struct {
		// Not a struct pointer, fall back to standard JSON unmarshaling
		if j.verboseDebug && j.logger != nil {
			j.logger.Debug("JSONFeeder: Not a struct pointer, using standard JSON unmarshaling", "structureType", reflect.TypeOf(structure))
		}
		if err := json.Unmarshal(data, structure); err != nil {
			return fmt.Errorf("failed to unmarshal JSON data: %w", err)
		}
		return nil
	}

	// Process the structure with field tracking
	return j.processStructFields(reflect.ValueOf(structure).Elem(), jsonData, "")
}

// processStructFields iterates through struct fields and populates them from JSON data
func (j *JSONFeeder) processStructFields(rv reflect.Value, jsonData map[string]interface{}, fieldPrefix string) error {
	structType := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get JSON tag or use field name
		jsonTag := fieldType.Tag.Get("json")
		var jsonKey string

		if jsonTag == "" {
			// No JSON tag, use field name
			jsonKey = fieldType.Name
		} else if jsonTag == "-" {
			// Explicitly skipped
			continue
		} else {
			// Handle json tag with options (e.g., "field,omitempty")
			jsonKey = strings.Split(jsonTag, ",")[0]
			if jsonKey == "" {
				jsonKey = fieldType.Name
			}
		}

		fieldPath := fieldType.Name // Use struct field name for path
		if fieldPrefix != "" {
			fieldPath = fieldPrefix + "." + fieldType.Name
		}

		// Check if this key exists in the JSON data
		if value, exists := jsonData[jsonKey]; exists {
			if err := j.processField(field, fieldType, value, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// processField processes a single field, handling nested structs, slices, and basic types
func (j *JSONFeeder) processField(field reflect.Value, fieldType reflect.StructField, value interface{}, fieldPath string) error {
	fieldKind := field.Kind()

	switch fieldKind {
	case reflect.Struct:
		// Handle nested structs
		if nestedMap, ok := value.(map[string]interface{}); ok {
			return j.processStructFields(field, nestedMap, fieldPath)
		}
		return wrapJSONMapError(fieldPath, value)

	case reflect.Slice:
		// Handle slices
		return j.setSliceFromJSON(field, value, fieldPath)

	case reflect.Map:
		// Handle maps
		if mapData, ok := value.(map[string]interface{}); ok {
			return j.setMapFromJSON(field, mapData, fieldType.Name, fieldPath)
		}
		return wrapJSONMapError(fieldPath, value)

	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.String,
		reflect.UnsafePointer:
		// Handle basic types and unsupported types
		return j.setFieldFromJSON(field, value, fieldPath)

	default:
		// Handle any remaining types
		return j.setFieldFromJSON(field, value, fieldPath)
	}
}

// setFieldFromJSON sets a field value from JSON data with type conversion
func (j *JSONFeeder) setFieldFromJSON(field reflect.Value, value interface{}, fieldPath string) error {
	// Convert and set the value
	convertedValue := reflect.ValueOf(value)
	if convertedValue.Type().ConvertibleTo(field.Type()) {
		field.Set(convertedValue.Convert(field.Type()))

		// Record field population
		if j.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:  fieldPath,
				FieldName:  fieldPath,
				FieldType:  field.Type().String(),
				FeederType: "JSONFeeder",
				SourceType: "json_file",
				SourceKey:  fieldPath,
				Value:      value,
				SearchKeys: []string{fieldPath},
				FoundKey:   fieldPath,
			}
			j.fieldTracker.RecordFieldPopulation(fp)
		}

		return nil
	}

	return wrapJSONConvertError(value, field.Type().String(), fieldPath)
}

// setSliceFromJSON sets a slice field from JSON array data
func (j *JSONFeeder) setSliceFromJSON(field reflect.Value, value interface{}, fieldPath string) error {
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
				return wrapJSONSliceElementError(item, elemType.String(), fieldPath, i)
			}
		}

		field.Set(newSlice)

		// Record field population for the slice
		if j.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:  fieldPath,
				FieldName:  fieldPath,
				FieldType:  field.Type().String(),
				FeederType: "JSONFeeder",
				SourceType: "json_file",
				SourceKey:  fieldPath,
				Value:      value,
				SearchKeys: []string{fieldPath},
				FoundKey:   fieldPath,
			}
			j.fieldTracker.RecordFieldPopulation(fp)
		}

		return nil
	}

	return wrapJSONArrayError(fieldPath, value)
}

// setMapFromJSON sets a map field value from JSON data with support for pointer and value types
func (j *JSONFeeder) setMapFromJSON(field reflect.Value, jsonData map[string]interface{}, fieldName, fieldPath string) error {
	if !field.CanSet() {
		return wrapJSONFieldCannotBeSet(fieldPath)
	}

	mapType := field.Type()
	keyType := mapType.Key()
	valueType := mapType.Elem()

	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Setting map from JSON", "fieldName", fieldName, "mapType", mapType, "keyType", keyType, "valueType", valueType)
	}

	// Create a new map
	newMap := reflect.MakeMap(mapType)

	// Handle different value types
	switch valueType.Kind() {
	case reflect.Struct:
		// Map of structs, like map[string]DBConnection
		for key, value := range jsonData {
			if valueMap, ok := value.(map[string]interface{}); ok {
				// Create a new instance of the struct type
				structValue := reflect.New(valueType).Elem()

				// Process the struct fields
				if err := j.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
					return fmt.Errorf("error processing map entry '%s': %w", key, err)
				}

				// Set the map entry
				keyValue := reflect.ValueOf(key)
				newMap.SetMapIndex(keyValue, structValue)
			} else {
				if j.verboseDebug && j.logger != nil {
					j.logger.Debug("JSONFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
				}
			}
		}
	case reflect.Ptr:
		// Map of pointers to structs, like map[string]*DBConnection
		elemType := valueType.Elem()
		if elemType.Kind() == reflect.Struct {
			for key, value := range jsonData {
				if value == nil {
					// Handle null values in JSON
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, reflect.Zero(valueType)) // Set to nil pointer
					if j.verboseDebug && j.logger != nil {
						j.logger.Debug("JSONFeeder: Set nil pointer for null JSON value", "key", key)
					}
				} else if valueMap, ok := value.(map[string]interface{}); ok {
					// Create a new instance of the struct type
					structValue := reflect.New(elemType).Elem()

					// Process the struct fields
					if err := j.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
						return fmt.Errorf("error processing map entry '%s': %w", key, err)
					}

					// Create a pointer to the struct and set the map entry
					ptrValue := reflect.New(elemType)
					ptrValue.Elem().Set(structValue)
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, ptrValue)

					if j.verboseDebug && j.logger != nil {
						j.logger.Debug("JSONFeeder: Successfully processed pointer to struct map entry", "key", key, "structType", elemType)
					}
				} else {
					if j.verboseDebug && j.logger != nil {
						j.logger.Debug("JSONFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
					}
				}
			}
		} else {
			// Map of pointers to non-struct types - handle direct conversion
			for key, value := range jsonData {
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
						if j.verboseDebug && j.logger != nil {
							j.logger.Debug("JSONFeeder: Cannot convert map value for pointer type", "key", key, "valueType", valueReflect.Type(), "targetType", elemType)
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
		for key, value := range jsonData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				if j.verboseDebug && j.logger != nil {
					j.logger.Debug("JSONFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
			}
		}
	default:
		// Map of primitive types - use direct conversion
		for key, value := range jsonData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				if j.verboseDebug && j.logger != nil {
					j.logger.Debug("JSONFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
			}
		}
	}

	// Set the field to the new map
	field.Set(newMap)

	// Record field population if tracker is available
	if j.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "JSONFeeder",
			SourceType:  "json_file",
			SourceKey:   fieldName, // For maps, use the field name as the source key
			Value:       field.Interface(),
			InstanceKey: "",
			SearchKeys:  []string{fieldName},
			FoundKey:    fieldName,
		}
		j.fieldTracker.RecordFieldPopulation(fp)
	}

	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Successfully set map field", "fieldName", fieldName, "mapSize", newMap.Len())
	}

	return nil
}
