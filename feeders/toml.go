package feeders

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/golobby/config/v3/pkg/feeder"
)

// TomlFeeder is a feeder that reads TOML files with optional verbose debug logging
type TomlFeeder struct {
	feeder.Toml
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewTomlFeeder creates a new TomlFeeder that reads from the specified TOML file
func NewTomlFeeder(filePath string) TomlFeeder {
	return TomlFeeder{
		Toml:         feeder.Toml{Path: filePath},
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
func (t TomlFeeder) Feed(structure interface{}) error {
	if t.verboseDebug && t.logger != nil {
		t.logger.Debug("TomlFeeder: Starting feed process", "filePath", t.Path, "structureType", reflect.TypeOf(structure))
	}

	var err error

	// Use field tracking if available
	if t.fieldTracker != nil {
		err = t.feedWithTracking(structure)
	} else {
		err = t.Toml.Feed(structure)
	}

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
func (t TomlFeeder) FeedKey(key string, target interface{}) error {
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
func (t TomlFeeder) feedWithTracking(structure interface{}) error {
	if t.fieldTracker == nil {
		// Fall back to regular feeding if no tracker is set
		return wrapTomlFeederError(t.Toml.Feed(structure))
	}

	// Read and parse the TOML file manually for field tracking
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return fmt.Errorf("failed to read TOML file %s: %w", t.Path, err)
	}

	var tomlData map[string]interface{}
	if err := toml.Unmarshal(data, &tomlData); err != nil {
		return fmt.Errorf("failed to parse TOML file %s: %w", t.Path, err)
	}

	// Process the structure with field tracking
	return t.processStructFields(reflect.ValueOf(structure).Elem(), tomlData, "")
}

// processStructFields iterates through struct fields and populates them from TOML data
func (t TomlFeeder) processStructFields(rv reflect.Value, tomlData map[string]interface{}, fieldPrefix string) error {
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
		if tomlTag == "" || tomlTag == "-" {
			continue
		}

		// Handle toml tag with options (e.g., "field,omitempty")
		tomlKey := strings.Split(tomlTag, ",")[0]
		if tomlKey == "" {
			tomlKey = fieldType.Name
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
func (t TomlFeeder) processField(field reflect.Value, fieldType reflect.StructField, value interface{}, fieldPath string) error {
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

	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.String,
		reflect.UnsafePointer:
		// Handle basic types and unsupported types
		return t.setFieldFromTOML(field, value, fieldPath)

	default:
		// Handle any remaining types
		return t.setFieldFromTOML(field, value, fieldPath)
	}
}

// setFieldFromTOML sets a field value from TOML data with type conversion
func (t TomlFeeder) setFieldFromTOML(field reflect.Value, value interface{}, fieldPath string) error {
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
func (t TomlFeeder) setSliceFromTOML(field reflect.Value, value interface{}, fieldPath string) error {
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
