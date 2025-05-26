package modular

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

const (
	// Struct tag keys
	tagDefault  = "default"
	tagRequired = "required"
	tagValidate = "validate"
	tagDesc     = "desc" // Used for generating sample config and documentation
)

// ConfigValidator is an interface for configuration validation
type ConfigValidator interface {
	// Validate validates the configuration and returns an error if invalid
	Validate() error
}

// ProcessConfigDefaults applies default values to a config struct based on struct tags
// It looks for `default:"value"` tags on struct fields and sets the field value if currently zero/empty
func ProcessConfigDefaults(cfg interface{}) error {
	if cfg == nil {
		return ErrConfigNil
	}

	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return ErrConfigNotPointer
	}

	v = v.Elem() // Dereference pointer
	if v.Kind() != reflect.Struct {
		return ErrConfigNotStruct
	}

	return processStructDefaults(v)
}

// processStructDefaults recursively processes struct fields for default values
func processStructDefaults(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle embedded structs
		if field.Kind() == reflect.Struct {
			if err := processStructDefaults(field); err != nil {
				return err
			}
			continue
		}

		// Handle pointers to structs - but only if they're already non-nil
		if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			// Don't automatically initialize nil struct pointers
			// (the previous behavior was automatically creating them)
			if !field.IsNil() {
				if err := processStructDefaults(field.Elem()); err != nil {
					return err
				}
			}
			continue
		}

		// Check for default tag
		defaultVal, hasDefault := fieldType.Tag.Lookup(tagDefault)
		if !hasDefault || !isZeroValue(field) {
			continue
		}

		// Set default value based on field type
		if err := setDefaultValue(field, defaultVal); err != nil {
			return fmt.Errorf("failed to set default value for %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// ValidateConfigRequired checks all struct fields with `required:"true"` tag
// and verifies they are not zero/empty values
func ValidateConfigRequired(cfg interface{}) error {
	if cfg == nil {
		return ErrConfigNil
	}

	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return ErrConfigNotPointer
	}

	v = v.Elem() // Dereference pointer
	if v.Kind() != reflect.Struct {
		return ErrConfigNotStruct
	}

	var requiredErrors []string
	validateRequiredFields(v, "", &requiredErrors)

	if len(requiredErrors) > 0 {
		return fmt.Errorf("%w: %s", ErrConfigRequiredFieldMissing, strings.Join(requiredErrors, ", "))
	}

	return nil
}

// validateRequiredFields recursively validates required fields
func validateRequiredFields(v reflect.Value, prefix string, errors *[]string) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		fieldName := fieldType.Name

		if prefix != "" {
			fieldName = prefix + "." + fieldName
		}

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle embedded structs
		if field.Kind() == reflect.Struct {
			validateRequiredFields(field, fieldName, errors)
			continue
		}

		// Handle pointers to structs
		if field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct {
			if !field.IsNil() {
				validateRequiredFields(field.Elem(), fieldName, errors)
			} else if isFieldRequired(&fieldType) {
				*errors = append(*errors, fieldName)
			}
			continue
		}

		// Check required tag
		if isFieldRequired(&fieldType) && isZeroValue(field) {
			*errors = append(*errors, fieldName)
		}
	}
}

// isFieldRequired checks if a field has the required:"true" tag
func isFieldRequired(field *reflect.StructField) bool {
	required, exists := field.Tag.Lookup(tagRequired)
	return exists && required == "true"
}

// isZeroValue determines if a field contains its zero value
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Invalid:
		return true
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	case reflect.Chan, reflect.Func, reflect.Struct, reflect.UnsafePointer:
		// Can't easily determine zero value for these types
		return false
	default:
		// For any other types not explicitly handled
		return false
	}
}

// setDefaultValue sets a default value from a string to the proper field type
func setDefaultValue(field reflect.Value, defaultVal string) error {
	kind := field.Kind()

	switch kind {
	case reflect.String:
		field.SetString(defaultVal)
		return nil
	case reflect.Bool:
		return setDefaultBool(field, defaultVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setDefaultIntValue(field, defaultVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return setDefaultUintValue(field, defaultVal)
	case reflect.Float32, reflect.Float64:
		return setDefaultFloatValue(field, defaultVal)
	case reflect.Slice:
		return setDefaultSlice(field, defaultVal)
	case reflect.Map:
		return setDefaultMap(field, defaultVal)
	case reflect.Invalid, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.Struct,
		reflect.UnsafePointer:
		return handleUnsupportedDefaultType(kind)
	default:
		return handleUnsupportedDefaultType(kind)
	}
}

// handleUnsupportedDefaultType returns appropriate errors for unsupported types
func handleUnsupportedDefaultType(kind reflect.Kind) error {
	switch kind {
	case reflect.Invalid:
		return fmt.Errorf("%w: invalid field", ErrUnsupportedTypeForDefault)
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.String, reflect.Map, reflect.Slice:
		return fmt.Errorf("%w: %s fields should be handled by setDefaultValue", ErrUnsupportedTypeForDefault, kind)
	case reflect.Complex64, reflect.Complex128:
		return fmt.Errorf("%w: complex numbers not supported", ErrUnsupportedTypeForDefault)
	case reflect.Array:
		return fmt.Errorf("%w: arrays not supported", ErrUnsupportedTypeForDefault)
	case reflect.Chan:
		return fmt.Errorf("%w: channels not supported", ErrUnsupportedTypeForDefault)
	case reflect.Func:
		return fmt.Errorf("%w: functions not supported", ErrUnsupportedTypeForDefault)
	case reflect.Interface:
		return fmt.Errorf("%w: interfaces not supported", ErrUnsupportedTypeForDefault)
	case reflect.Ptr:
		return fmt.Errorf("%w: pointers not supported", ErrUnsupportedTypeForDefault)
	case reflect.Struct:
		return fmt.Errorf("%w: structs not supported", ErrUnsupportedTypeForDefault)
	case reflect.UnsafePointer:
		return fmt.Errorf("%w: unsafe pointers not supported", ErrUnsupportedTypeForDefault)
	default:
		return fmt.Errorf("%w: unknown type %s", ErrUnsupportedTypeForDefault, kind)
	}
}

func setDefaultBool(field reflect.Value, defaultVal string) error {
	b, err := strconv.ParseBool(defaultVal)
	if err != nil {
		return fmt.Errorf("failed to parse bool value: %w", err)
	}
	field.SetBool(b)
	return nil
}

// setDefaultIntValue parses and sets an integer default value
func setDefaultIntValue(field reflect.Value, defaultVal string) error {
	i, err := strconv.ParseInt(defaultVal, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse int value: %w", err)
	}
	return setDefaultInt(field, i)
}

// setDefaultUintValue parses and sets an unsigned integer default value
func setDefaultUintValue(field reflect.Value, defaultVal string) error {
	u, err := strconv.ParseUint(defaultVal, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse uint value: %w", err)
	}
	return setDefaultUint(field, u)
}

// setDefaultFloatValue parses and sets a float default value
func setDefaultFloatValue(field reflect.Value, defaultVal string) error {
	f, err := strconv.ParseFloat(defaultVal, 64)
	if err != nil {
		return fmt.Errorf("failed to parse float value: %w", err)
	}
	return setDefaultFloat(field, f)
}

// setDefaultSlice sets a slice default value from JSON
func setDefaultSlice(field reflect.Value, defaultVal string) error {
	// Attempt to unmarshal JSON array into slice
	if field.Type().Elem().Kind() == reflect.String {
		var strs []string
		if err := json.Unmarshal([]byte(defaultVal), &strs); err != nil {
			return fmt.Errorf("failed to unmarshal JSON array: %w", err)
		}
		sliceVal := reflect.MakeSlice(field.Type(), len(strs), len(strs))
		for i, s := range strs {
			sliceVal.Index(i).SetString(s)
		}
		field.Set(sliceVal)
	}
	return nil
}

// setDefaultMap sets a map default value from JSON
func setDefaultMap(field reflect.Value, defaultVal string) error {
	// Only handle string->string maps for defaults
	if field.Type().Key().Kind() == reflect.String && field.Type().Elem().Kind() == reflect.String {
		var m map[string]string
		if err := json.Unmarshal([]byte(defaultVal), &m); err != nil {
			return fmt.Errorf("failed to unmarshal JSON map: %w", err)
		}
		mapVal := reflect.MakeMap(field.Type())
		for k, v := range m {
			mapVal.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
		}
		field.Set(mapVal)
	}
	return nil
}

func setDefaultInt(field reflect.Value, i int64) error {
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.OverflowInt(i) {
			return fmt.Errorf("%w: %d overflows %s", ErrDefaultValueOverflowsInt, i, field.Type())
		}
		field.SetInt(i)
		return nil
	case reflect.Invalid, reflect.Bool, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Ptr, reflect.Slice, reflect.String, reflect.Struct, reflect.UnsafePointer:
		return fmt.Errorf("%w: cannot set int value to %s", ErrIncompatibleFieldKind, field.Kind())
	default:
		return fmt.Errorf("%w: cannot set int value to %s", ErrIncompatibleFieldKind, field.Kind())
	}
}

func setDefaultUint(field reflect.Value, u uint64) error {
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if field.OverflowUint(u) {
			return fmt.Errorf("%w: %d overflows %s", ErrDefaultValueOverflowsUint, u, field.Type())
		}
		field.SetUint(u)
		return nil
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr,
		reflect.Slice, reflect.String, reflect.Struct, reflect.UnsafePointer:
		return fmt.Errorf("%w: cannot set uint value to %s", ErrIncompatibleFieldKind, field.Kind())
	default:
		return fmt.Errorf("%w: cannot set uint value to %s", ErrIncompatibleFieldKind, field.Kind())
	}
}

// setDefaultFloat sets a default float value to a field, checking for overflow
func setDefaultFloat(field reflect.Value, f float64) error {
	switch field.Kind() {
	case reflect.Float32, reflect.Float64:
		if field.OverflowFloat(f) {
			return fmt.Errorf("%w: %f overflows %s", ErrDefaultValueOverflowsFloat, f, field.Type())
		}
		field.SetFloat(f)
		return nil
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan,
		reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.String,
		reflect.Struct, reflect.UnsafePointer:
		return fmt.Errorf("%w: cannot set float value to %s", ErrIncompatibleFieldKind, field.Kind())
	default:
		return fmt.Errorf("%w: cannot set float value to %s", ErrIncompatibleFieldKind, field.Kind())
	}
}

// GenerateSampleConfig generates a sample configuration for a config struct
// The format parameter can be "yaml", "json", or "toml"
func GenerateSampleConfig(cfg interface{}, format string) ([]byte, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	// Apply any default values to the sample config
	sampleConfig := reflect.New(reflect.TypeOf(cfg).Elem()).Interface()
	if err := ProcessConfigDefaults(sampleConfig); err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "yaml":
		data, err := yaml.Marshal(sampleConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		return data, nil
	case "json":
		// Handle JSON field name mapping based on struct tags
		jsonFields := mapStructFieldsForJSON(sampleConfig)
		data, err := json.MarshalIndent(jsonFields, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		return data, nil
	case "toml":
		var buf strings.Builder
		if err := toml.NewEncoder(&buf).Encode(sampleConfig); err != nil {
			return nil, fmt.Errorf("failed to marshal to TOML: %w", err)
		}
		return []byte(buf.String()), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormatType, format)
	}
}

// mapStructFieldsForJSON creates a map with proper JSON field names based on struct tags
func mapStructFieldsForJSON(cfg interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Get the json field name from tag
		jsonTag := fieldType.Tag.Get("json")
		yamlTag := fieldType.Tag.Get("yaml")

		// Determine the field name to use
		fieldName := fieldType.Name
		if jsonTag != "" && jsonTag != "-" {
			// Use the json tag name if present
			parts := strings.Split(jsonTag, ",")
			fieldName = parts[0]
		} else if yamlTag != "" && yamlTag != "-" {
			// Fall back to yaml tag if no json tag
			parts := strings.Split(yamlTag, ",")
			fieldName = parts[0]
		}

		// Convert nested structs recursively
		if field.Kind() == reflect.Struct {
			result[fieldName] = mapStructFieldsForJSON(field.Interface())
		} else if field.Kind() == reflect.Ptr && !field.IsNil() && field.Elem().Kind() == reflect.Struct {
			result[fieldName] = mapStructFieldsForJSON(field.Interface())
		} else {
			// Handle primitive types and other values
			result[fieldName] = field.Interface()
		}
	}

	return result
}

// SaveSampleConfig generates and saves a sample configuration file
func SaveSampleConfig(cfg interface{}, format, filePath string) error {
	data, err := GenerateSampleConfig(cfg, format)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file to %s: %w", filePath, err)
	}
	return nil
}

// ValidateConfig validates a configuration using the following steps:
// 1. Processes default values
// 2. Validates required fields
// 3. If the config implements ConfigValidator, calls its Validate method
func ValidateConfig(cfg interface{}) error {
	if cfg == nil {
		return ErrConfigNil
	}

	// Apply default values
	if err := ProcessConfigDefaults(cfg); err != nil {
		return err
	}

	// Check required fields
	if err := ValidateConfigRequired(cfg); err != nil {
		return err
	}

	// Custom validation if implements ConfigValidator
	if validator, ok := cfg.(ConfigValidator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}
	}

	return nil
}
