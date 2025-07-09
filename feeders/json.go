package feeders

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/golobby/config/v3/pkg/feeder"
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
	feeder.Json
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewJSONFeeder creates a new JSONFeeder that reads from the specified JSON file
func NewJSONFeeder(filePath string) JSONFeeder {
	return JSONFeeder{
		Json:         feeder.Json{Path: filePath},
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
func (j JSONFeeder) Feed(structure interface{}) error {
	if j.verboseDebug && j.logger != nil {
		j.logger.Debug("JSONFeeder: Starting feed process", "filePath", j.Path, "structureType", reflect.TypeOf(structure))
	}

	var err error

	// Use field tracking if available
	if j.fieldTracker != nil {
		err = j.feedWithTracking(structure)
	} else {
		err = j.Json.Feed(structure)
	}

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
func (j JSONFeeder) FeedKey(key string, target interface{}) error {
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
func (j JSONFeeder) feedWithTracking(structure interface{}) error {
	if j.fieldTracker == nil {
		// Fall back to regular feeding if no tracker is set
		return wrapJsonFeederError(j.Json.Feed(structure))
	}

	// Read and parse the JSON file manually for field tracking
	data, err := os.ReadFile(j.Path)
	if err != nil {
		return fmt.Errorf("failed to read JSON file %s: %w", j.Path, err)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to parse JSON file %s: %w", j.Path, err)
	}

	// Process the structure with field tracking
	return j.processStructFields(reflect.ValueOf(structure).Elem(), jsonData, "")
}

// processStructFields iterates through struct fields and populates them from JSON data
func (j JSONFeeder) processStructFields(rv reflect.Value, jsonData map[string]interface{}, fieldPrefix string) error {
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
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Handle json tag with options (e.g., "field,omitempty")
		jsonKey := strings.Split(jsonTag, ",")[0]
		if jsonKey == "" {
			jsonKey = fieldType.Name
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
func (j JSONFeeder) processField(field reflect.Value, fieldType reflect.StructField, value interface{}, fieldPath string) error {
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

	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.String,
		reflect.UnsafePointer:
		// Handle basic types and unsupported types
		return j.setFieldFromJSON(field, value, fieldPath)

	default:
		// Handle any remaining types
		return j.setFieldFromJSON(field, value, fieldPath)
	}
}

// setFieldFromJSON sets a field value from JSON data with type conversion
func (j JSONFeeder) setFieldFromJSON(field reflect.Value, value interface{}, fieldPath string) error {
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
func (j JSONFeeder) setSliceFromJSON(field reflect.Value, value interface{}, fieldPath string) error {
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
