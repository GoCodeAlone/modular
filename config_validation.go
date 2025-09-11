package modular

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Static errors for config validation
var (
	ErrConfigsNil = errors.New("configs cannot be nil")
)

const (
	// Struct tag keys
	tagDefault  = "default"
	tagRequired = "required"
	tagValidate = "validate"
	tagDesc     = "desc"    // Used for generating sample config and documentation
	tagDynamic  = "dynamic" // Used for dynamic reload functionality
)

// ConfigValidator is an interface for configuration validation.
// Configuration structs can implement this interface to provide
// custom validation logic beyond the standard required field checking.
//
// The framework automatically calls Validate() on configuration objects
// that implement this interface during module initialization.
//
// Example implementation:
//
//	type MyConfig struct {
//	    Host string `json:"host" required:"true"`
//	    Port int    `json:"port" default:"8080" validate:"range:1024-65535"`
//	}
//
//	func (c *MyConfig) Validate() error {
//	    if c.Port < 1024 || c.Port > 65535 {
//	        return fmt.Errorf("port must be between 1024 and 65535")
//	    }
//	    return nil
//	}
type ConfigValidator interface {
	// Validate validates the configuration and returns an error if invalid.
	// This method is called automatically by the framework after configuration
	// loading and default value processing. It should return a descriptive
	// error if the configuration is invalid.
	Validate() error
}

// ProcessConfigDefaults applies default values to a config struct based on struct tags.
// It looks for `default:"value"` tags on struct fields and sets the field value if currently zero/empty.
//
// Supported field types:
//   - Basic types: string, int, float, bool
//   - Slices: []string, []int, etc.
//   - Pointers to basic types
//
// Example struct tags:
//
//	type Config struct {
//	    Host     string `default:"localhost"`
//	    Port     int    `default:"8080"`
//	    Debug    bool   `default:"false"`
//	    Features []string `default:"feature1,feature2"`
//	}
//
// This function is automatically called by the configuration loading system
// before validation, but can also be called manually if needed.
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
	// Special handling for time.Duration type
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		return setDefaultDuration(field, defaultVal)
	}

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

// setDefaultDuration parses and sets a duration default value
func setDefaultDuration(field reflect.Value, defaultVal string) error {
	d, err := time.ParseDuration(defaultVal)
	if err != nil {
		return fmt.Errorf("failed to parse duration value: %w", err)
	}
	field.SetInt(int64(d))
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
		switch field.Kind() { //nolint:exhaustive // only handling specific cases we care about
		case reflect.Struct:
			result[fieldName] = mapStructFieldsForJSON(field.Interface())
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				result[fieldName] = mapStructFieldsForJSON(field.Interface())
			} else {
				result[fieldName] = field.Interface()
			}
		default:
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

// DynamicFieldParser interface defines how dynamic field detection works
// for configuration reload functionality according to T044 requirements
type DynamicFieldParser interface {
	// GetDynamicFields analyzes a configuration struct and returns a slice
	// of field names that are tagged with `dynamic:"true"`
	GetDynamicFields(config interface{}) ([]string, error)

	// ValidateDynamicReload compares two configurations and generates a ConfigDiff
	// that only includes changes to fields marked as dynamic
	ValidateDynamicReload(oldConfig, newConfig interface{}) (*ConfigDiff, error)
}

// StdDynamicFieldParser implements DynamicFieldParser using reflection
type StdDynamicFieldParser struct{}

// NewDynamicFieldParser creates a new standard dynamic field parser
func NewDynamicFieldParser() DynamicFieldParser {
	return &StdDynamicFieldParser{}
}

// GetDynamicFields parses a config struct and returns dynamic field names
func (p *StdDynamicFieldParser) GetDynamicFields(config interface{}) ([]string, error) {
	if config == nil {
		return nil, ErrConfigNil
	}

	value := reflect.ValueOf(config)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, ErrConfigNil
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: got %v", ErrConfigNotStruct, value.Kind())
	}

	var dynamicFields []string
	p.parseDynamicFields(value, "", &dynamicFields)

	return dynamicFields, nil
}

// parseDynamicFields recursively traverses struct fields to find dynamic tags
func (p *StdDynamicFieldParser) parseDynamicFields(value reflect.Value, prefix string, fields *[]string) {
	structType := value.Type()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := value.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		fieldPath := field.Name
		if prefix != "" {
			fieldPath = prefix + "." + field.Name
		}

		// Check for dynamic tag
		if dynamicTag := field.Tag.Get(tagDynamic); dynamicTag == "true" {
			*fields = append(*fields, fieldPath)
		}

		// Recursively handle nested structs
		if fieldValue.Kind() == reflect.Struct {
			p.parseDynamicFields(fieldValue, fieldPath, fields)
		} else if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			if fieldValue.Elem().Kind() == reflect.Struct {
				p.parseDynamicFields(fieldValue.Elem(), fieldPath, fields)
			}
		}
	}
}

// ValidateDynamicReload compares configs and creates a diff with only dynamic changes
func (p *StdDynamicFieldParser) ValidateDynamicReload(oldConfig, newConfig interface{}) (*ConfigDiff, error) {
	if oldConfig == nil || newConfig == nil {
		return nil, ErrConfigsNil
	}

	// Get dynamic fields from the new config (should be the same for both)
	dynamicFields, err := p.GetDynamicFields(newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic fields: %w", err)
	}

	// Create a set for faster lookup
	dynamicFieldsSet := make(map[string]bool)
	for _, field := range dynamicFields {
		dynamicFieldsSet[field] = true
	}

	// Get all field values from both configs
	oldValues, err := p.getFieldValues(oldConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get old config values: %w", err)
	}

	newValues, err := p.getFieldValues(newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get new config values: %w", err)
	}

	// Create diff with only dynamic field changes
	diff := &ConfigDiff{
		Changed:   make(map[string]FieldChange),
		Added:     make(map[string]interface{}),
		Removed:   make(map[string]interface{}),
		Timestamp: time.Now(),
		DiffID:    fmt.Sprintf("dynamic-reload-%d", time.Now().UnixNano()),
	}

	// Check for changes in dynamic fields only
	for fieldPath := range dynamicFieldsSet {
		oldVal, oldExists := oldValues[fieldPath]
		newVal, newExists := newValues[fieldPath]

		if !oldExists && newExists {
			// Field added
			diff.Added[fieldPath] = newVal
		} else if oldExists && !newExists {
			// Field removed
			diff.Removed[fieldPath] = oldVal
		} else if oldExists && newExists {
			// Check if value changed
			if !reflect.DeepEqual(oldVal, newVal) {
				diff.Changed[fieldPath] = FieldChange{
					FieldPath:   fieldPath,
					OldValue:    oldVal,
					NewValue:    newVal,
					ChangeType:  ChangeTypeModified,
					IsSensitive: false, // Could be enhanced to detect sensitive fields
				}
			}
		}
	}

	return diff, nil
}

// getFieldValues extracts all field values from a config struct as a flat map
func (p *StdDynamicFieldParser) getFieldValues(config interface{}) (map[string]interface{}, error) {
	values := make(map[string]interface{})

	value := reflect.ValueOf(config)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil, ErrConfigNotStruct
	}

	p.extractFieldValues(value, "", values)
	return values, nil
}

// extractFieldValues recursively extracts field values into a flat map
func (p *StdDynamicFieldParser) extractFieldValues(value reflect.Value, prefix string, values map[string]interface{}) {
	structType := value.Type()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := value.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		fieldPath := field.Name
		if prefix != "" {
			fieldPath = prefix + "." + field.Name
		}

		// Handle different field types
		if fieldValue.Kind() == reflect.Struct {
			p.extractFieldValues(fieldValue, fieldPath, values)
		} else if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			if fieldValue.Elem().Kind() == reflect.Struct {
				p.extractFieldValues(fieldValue.Elem(), fieldPath, values)
			} else {
				values[fieldPath] = fieldValue.Interface()
			}
		} else {
			values[fieldPath] = fieldValue.Interface()
		}
	}
}
