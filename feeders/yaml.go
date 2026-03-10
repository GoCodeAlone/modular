package feeders

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// wrapDebugLogger returns a func(string) that calls the logger's Debug method.
// This indirection avoids go vet false positives about non-constant format strings
// passed to Debug(msg string, args ...any) interface methods.
//
//go:noinline
func wrapDebugLogger(logger interface{ Debug(msg string, args ...any) }) func(string) {
	debug := logger.Debug
	return func(msg string) { debug(msg) }
}

// parseYAMLTag parses a YAML struct tag and returns the field name and options
func parseYAMLTag(tag string) (fieldName string, options []string) {
	if tag == "" {
		return "", nil
	}

	parts := strings.Split(tag, ",")
	fieldName = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		options = make([]string, len(parts)-1)
		for i, opt := range parts[1:] {
			options[i] = strings.TrimSpace(opt)
		}
	}

	return fieldName, options
}

// getFieldNameFromTag extracts the field name from YAML tag or falls back to struct field name
func getFieldNameFromTag(fieldType *reflect.StructField) (string, bool) {
	if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
		fieldName, _ := parseYAMLTag(yamlTag)
		if fieldName == "" {
			fieldName = fieldType.Name
		}
		return fieldName, true
	}
	return "", false
}

// YamlFeeder is a feeder that reads YAML files with optional verbose debug logging
type YamlFeeder struct {
	Path         string
	verboseDebug bool
	debugFn      func(string)
	fieldTracker FieldTracker
	priority     int
}

// NewYamlFeeder creates a new YamlFeeder that reads from the specified YAML file
func NewYamlFeeder(filePath string) *YamlFeeder {
	return &YamlFeeder{
		Path:         filePath,
		verboseDebug: false,
		debugFn:      nil,
		fieldTracker: nil,
		priority:     0, // Default priority
	}
}

// WithPriority sets the priority for this feeder and returns the feeder for chaining.
// Higher priority values mean the feeder will be applied later, allowing it to override
// values from lower priority feeders.
func (y *YamlFeeder) WithPriority(priority int) *YamlFeeder {
	y.priority = priority
	return y
}

// Priority returns the priority value for this feeder.
func (y *YamlFeeder) Priority() int {
	return y.priority
}

// SetVerboseDebug enables or disables verbose debug logging
func (y *YamlFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	y.verboseDebug = enabled
	if logger != nil {
		y.debugFn = wrapDebugLogger(logger)
	} else {
		y.debugFn = nil
	}
	if enabled && logger != nil {
		logger.Debug("Verbose YAML feeder debugging enabled")
	}
}

// SetFieldTracker sets the field tracker for recording field populations
func (y *YamlFeeder) SetFieldTracker(tracker FieldTracker) {
	y.fieldTracker = tracker
}

// debugLog logs a debug message with key-value pairs when verbose debugging is enabled.
// Key-value pairs are formatted into the message string to avoid go vet printf false positives
// on the Debug(msg string, args ...any) interface method signature.
func (y *YamlFeeder) debugLog(msg string, keysAndValues ...any) {
	if !y.verboseDebug || y.debugFn == nil {
		return
	}
	if len(keysAndValues) == 0 {
		y.debugFn(msg)
		return
	}
	var b strings.Builder
	b.WriteString(msg)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		fmt.Fprintf(&b, " %v=%v", keysAndValues[i], keysAndValues[i+1])
	}
	if len(keysAndValues)%2 != 0 {
		fmt.Fprintf(&b, " %v", keysAndValues[len(keysAndValues)-1])
	}
	y.debugFn(b.String())
}

// Feed reads the YAML file and populates the provided structure
func (y *YamlFeeder) Feed(structure interface{}) error {
	y.debugLog("YamlFeeder: Starting feed process", "filePath", y.Path, "structureType", reflect.TypeOf(structure))

	// Always use custom parsing logic for consistency
	err := y.feedWithTracking(structure)

	if err != nil {
		y.debugLog("YamlFeeder: Feed completed with error", "filePath", y.Path, "error", err)
	} else {
		y.debugLog("YamlFeeder: Feed completed successfully", "filePath", y.Path)
	}
	if err != nil {
		return fmt.Errorf("yaml feed error: %w", err)
	}
	return nil
}

// FeedKey reads a YAML file and extracts a specific key
func (y *YamlFeeder) FeedKey(key string, target interface{}) error {
	y.debugLog("YamlFeeder: Starting FeedKey process", "filePath", y.Path, "key", key, "targetType", reflect.TypeOf(target))

	// Create a temporary map to hold all YAML data
	var allData map[interface{}]interface{}

	// Use the embedded Yaml feeder to read the file
	if err := y.Feed(&allData); err != nil {
		y.debugLog("YamlFeeder: Failed to read YAML file", "filePath", y.Path, "error", err)
		return fmt.Errorf("failed to read YAML: %w", err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		y.debugLog("YamlFeeder: Key not found in YAML file", "filePath", y.Path, "key", key)
		return nil
	}

	y.debugLog("YamlFeeder: Found key in YAML file", "filePath", y.Path, "key", key, "valueType", reflect.TypeOf(value))

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		y.debugLog("YamlFeeder: Failed to marshal value", "filePath", y.Path, "key", key, "error", err)
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err = yaml.Unmarshal(valueBytes, target); err != nil {
		y.debugLog("YamlFeeder: Failed to unmarshal value to target", "filePath", y.Path, "key", key, "error", err)
		return fmt.Errorf("failed to unmarshal value to target: %w", err)
	}

	y.debugLog("YamlFeeder: FeedKey completed successfully", "filePath", y.Path, "key", key)
	return nil
}

// feedWithTracking processes YAML data with field tracking support
func (y *YamlFeeder) feedWithTracking(structure interface{}) error {
	y.debugLog("YamlFeeder: Starting feedWithTracking", "filePath", y.Path, "structureType", reflect.TypeOf(structure))

	// Read YAML file
	content, err := os.ReadFile(y.Path)
	if err != nil {
		y.debugLog("YamlFeeder: Failed to read YAML file", "filePath", y.Path, "error", err)
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	// Check if we're dealing with a struct pointer
	structValue := reflect.ValueOf(structure)
	if structValue.Kind() != reflect.Ptr || structValue.Elem().Kind() != reflect.Struct {
		// Not a struct pointer, fall back to standard YAML unmarshaling
		y.debugLog("YamlFeeder: Not a struct pointer, using standard YAML unmarshaling", "structureType", reflect.TypeOf(structure))
		if err := yaml.Unmarshal(content, structure); err != nil {
			return fmt.Errorf("failed to unmarshal YAML data: %w", err)
		}
		return nil
	}

	// Parse YAML content
	data := make(map[string]interface{})
	if err := yaml.Unmarshal(content, &data); err != nil {
		y.debugLog("YamlFeeder: Failed to parse YAML content", "filePath", y.Path, "error", err)
		return fmt.Errorf("failed to parse YAML content: %w", err)
	}

	// Process the structure fields with tracking
	return y.processStructFields(reflect.ValueOf(structure).Elem(), data, "")
}

// processStructFields processes struct fields and tracks field populations from YAML data
func (y *YamlFeeder) processStructFields(rv reflect.Value, data map[string]interface{}, parentPath string) error {
	structType := rv.Type()

	y.debugLog("YamlFeeder: Processing struct fields", "structType", structType, "numFields", rv.NumField(), "parentPath", parentPath)

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Build field path
		fieldPath := fieldType.Name
		if parentPath != "" {
			fieldPath = parentPath + "." + fieldType.Name
		}

		y.debugLog("YamlFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldPath", fieldPath)

		if err := y.processField(field, &fieldType, data, fieldPath); err != nil {
			y.debugLog("YamlFeeder: Field processing failed", "fieldName", fieldType.Name, "error", err)
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}
	}
	return nil
}

// processField handles a single struct field with YAML data and field tracking
func (y *YamlFeeder) processField(field reflect.Value, fieldType *reflect.StructField, data map[string]interface{}, fieldPath string) error {
	// Get field name from YAML tag or use struct field name
	fieldName, hasYAMLTag := getFieldNameFromTag(fieldType)

	switch field.Kind() {
	case reflect.Ptr:
		// Handle pointer types
		if hasYAMLTag {
			return y.setPointerFromYAML(field, fieldName, data, fieldType.Name, fieldPath)
		}
	case reflect.Slice:
		// Handle slice types
		if hasYAMLTag {
			return y.setSliceFromYAML(field, fieldName, data, fieldType.Name, fieldPath)
		}
	case reflect.Array:
		// Handle array types
		if hasYAMLTag {
			return y.setArrayFromYAML(field, fieldName, data, fieldType.Name, fieldPath)
		}
	case reflect.Map:
		y.debugLog("YamlFeeder: Processing map field", "fieldName", fieldType.Name, "fieldPath", fieldPath)

		if hasYAMLTag {
			// Look for map data using the parsed field name
			if mapData, found := data[fieldName]; found {
				if mapDataTyped, ok := mapData.(map[string]interface{}); ok {
					return y.setMapFromYaml(field, mapDataTyped, fieldType.Name, fieldPath)
				} else {
					y.debugLog("YamlFeeder: Map YAML data is not a map[string]interface{}", "fieldName", fieldType.Name, "parsedFieldName", fieldName, "dataType", reflect.TypeOf(mapData))
				}
			} else {
				y.debugLog("YamlFeeder: Map YAML data not found", "fieldName", fieldType.Name, "parsedFieldName", fieldName)
			}
		}
	case reflect.Struct:
		y.debugLog("YamlFeeder: Processing nested struct", "fieldName", fieldType.Name, "fieldPath", fieldPath)

		if hasYAMLTag {
			// Look for nested data using the parsed field name
			if nestedData, found := data[fieldName]; found {
				if nestedMap, ok := nestedData.(map[string]interface{}); ok {
					return y.processStructFields(field, nestedMap, fieldPath)
				} else {
					y.debugLog("YamlFeeder: Nested YAML data is not a map", "fieldName", fieldType.Name, "parsedFieldName", fieldName, "dataType", reflect.TypeOf(nestedData))
				}
			} else {
				y.debugLog("YamlFeeder: Nested YAML data not found", "fieldName", fieldType.Name, "parsedFieldName", fieldName)
			}
		} else {
			// No yaml tag, use the same data map
			return y.processStructFields(field, data, fieldPath)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.String, reflect.UnsafePointer:
		// Check for yaml tag for primitive types and other non-struct types
		if hasYAMLTag {
			y.debugLog("YamlFeeder: Found yaml tag", "fieldName", fieldType.Name, "parsedFieldName", fieldName, "fieldPath", fieldPath)
			return y.setFieldFromYaml(field, fieldName, data, fieldType.Name, fieldPath)
		}
		y.debugLog("YamlFeeder: No yaml tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
	default:
		// Check for yaml tag for primitive types and other non-struct types
		if hasYAMLTag {
			y.debugLog("YamlFeeder: Found yaml tag", "fieldName", fieldType.Name, "parsedFieldName", fieldName, "fieldPath", fieldPath)
			return y.setFieldFromYaml(field, fieldName, data, fieldType.Name, fieldPath)
		}
		y.debugLog("YamlFeeder: No yaml tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
	}

	return nil
}

// setPointerFromYAML handles setting pointer fields from YAML data
func (y *YamlFeeder) setPointerFromYAML(field reflect.Value, yamlTag string, data map[string]interface{}, fieldName, fieldPath string) error {
	// Find the value in YAML data
	foundValue, exists := data[yamlTag]

	if !exists {
		// Record that we searched but didn't find
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  []string{yamlTag},
				FoundKey:    "",
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}
		return nil
	}

	if foundValue == nil {
		// Set nil pointer
		field.Set(reflect.Zero(field.Type()))

		// Record field population
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   yamlTag,
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  []string{yamlTag},
				FoundKey:    yamlTag,
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}
		return nil
	}

	// Create a new instance of the pointed-to type
	elemType := field.Type().Elem()
	ptrValue := reflect.New(elemType)

	// Handle different element types
	switch elemType.Kind() { //nolint:exhaustive // default case handles all other types
	case reflect.Struct:
		// Handle pointer to struct
		if valueMap, ok := foundValue.(map[string]interface{}); ok {
			if err := y.processStructFields(ptrValue.Elem(), valueMap, fieldPath); err != nil {
				return fmt.Errorf("error processing pointer to struct: %w", err)
			}
			field.Set(ptrValue)
		} else {
			return wrapYamlExpectedMapError(fieldPath, foundValue)
		}
	default:
		// Handle pointer to basic type
		if err := y.setFieldValue(ptrValue.Elem(), foundValue); err != nil {
			return fmt.Errorf("error setting pointer value: %w", err)
		}
		field.Set(ptrValue)
	}

	// Record field population
	if y.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "*feeders.YamlFeeder",
			SourceType:  "yaml",
			SourceKey:   yamlTag,
			Value:       foundValue,
			InstanceKey: "",
			SearchKeys:  []string{yamlTag},
			FoundKey:    yamlTag,
		}
		y.fieldTracker.RecordFieldPopulation(fp)
	}
	return nil
}

// setSliceFromYAML handles setting slice fields from YAML data
func (y *YamlFeeder) setSliceFromYAML(field reflect.Value, yamlTag string, data map[string]interface{}, fieldName, fieldPath string) error {
	// Find the value in YAML data
	foundValue, exists := data[yamlTag]

	if !exists {
		// Record that we searched but didn't find
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  []string{yamlTag},
				FoundKey:    "",
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}
		return nil
	}

	// Handle slice values
	arrayValue, ok := foundValue.([]interface{})
	if !ok {
		return wrapYamlExpectedArrayError(fieldPath, foundValue)
	}

	sliceType := field.Type()
	elemType := sliceType.Elem()

	newSlice := reflect.MakeSlice(sliceType, len(arrayValue), len(arrayValue))

	for i, item := range arrayValue {
		elem := newSlice.Index(i)

		// Handle different element types
		switch elemType.Kind() { //nolint:exhaustive // default case handles all other types
		case reflect.Struct:
			// Handle slice of structs
			if itemMap, ok := item.(map[string]interface{}); ok {
				if err := y.processStructFields(elem, itemMap, fmt.Sprintf("%s[%d]", fieldPath, i)); err != nil {
					return fmt.Errorf("error processing slice element %d: %w", i, err)
				}
			} else {
				return wrapYamlExpectedMapForSliceError(fieldPath, i, item)
			}
		case reflect.Ptr:
			// Handle slice of pointers
			if item == nil {
				// Set nil pointer
				elem.Set(reflect.Zero(elemType))
			} else if ptrElemType := elemType.Elem(); ptrElemType.Kind() == reflect.Struct {
				// Pointer to struct
				if itemMap, ok := item.(map[string]interface{}); ok {
					ptrValue := reflect.New(ptrElemType)
					if err := y.processStructFields(ptrValue.Elem(), itemMap, fmt.Sprintf("%s[%d]", fieldPath, i)); err != nil {
						return fmt.Errorf("error processing slice pointer element %d: %w", i, err)
					}
					elem.Set(ptrValue)
				} else {
					return wrapYamlExpectedMapForSliceError(fieldPath, i, item)
				}
			} else {
				// Pointer to basic type
				ptrValue := reflect.New(ptrElemType)
				if err := y.setFieldValue(ptrValue.Elem(), item); err != nil {
					return fmt.Errorf("error setting slice pointer element %d: %w", i, err)
				}
				elem.Set(ptrValue)
			}
		default:
			// Handle basic types
			if err := y.setFieldValue(elem, item); err != nil {
				return fmt.Errorf("error setting slice element %d: %w", i, err)
			}
		}
	}

	field.Set(newSlice)

	// Record field population for the slice
	if y.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "*feeders.YamlFeeder",
			SourceType:  "yaml",
			SourceKey:   yamlTag,
			Value:       foundValue,
			InstanceKey: "",
			SearchKeys:  []string{yamlTag},
			FoundKey:    yamlTag,
		}
		y.fieldTracker.RecordFieldPopulation(fp)
	}

	return nil
}

// setArrayFromYAML handles setting array fields from YAML data
func (y *YamlFeeder) setArrayFromYAML(field reflect.Value, yamlTag string, data map[string]interface{}, fieldName, fieldPath string) error {
	// Find the value in YAML data
	foundValue, exists := data[yamlTag]

	if !exists {
		// Record that we searched but didn't find
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  []string{yamlTag},
				FoundKey:    "",
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}
		return nil
	}

	// Handle array values
	arrayValue, ok := foundValue.([]interface{})
	if !ok {
		return wrapYamlExpectedArrayError(fieldPath, foundValue)
	}

	arrayType := field.Type()
	arrayLen := arrayType.Len()

	if len(arrayValue) > arrayLen {
		return wrapYamlArraySizeExceeded(fieldPath, len(arrayValue), arrayLen)
	}

	for i, item := range arrayValue {
		elem := field.Index(i)
		if err := y.setFieldValue(elem, item); err != nil {
			return fmt.Errorf("error setting array element %d: %w", i, err)
		}
	}

	// Record field population for the array
	if y.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "*feeders.YamlFeeder",
			SourceType:  "yaml",
			SourceKey:   yamlTag,
			Value:       foundValue,
			InstanceKey: "",
			SearchKeys:  []string{yamlTag},
			FoundKey:    yamlTag,
		}
		y.fieldTracker.RecordFieldPopulation(fp)
	}

	return nil
}

// setFieldFromYaml sets a field value from YAML data with field tracking
func (y *YamlFeeder) setFieldFromYaml(field reflect.Value, yamlTag string, data map[string]interface{}, fieldName, fieldPath string) error {
	// Find the value in YAML data
	searchKeys := []string{yamlTag}
	var foundValue interface{}
	var foundKey string

	if value, exists := data[yamlTag]; exists {
		foundValue = value
		foundKey = yamlTag
		y.debugLog("YamlFeeder: Found YAML value", "fieldName", fieldName, "yamlKey", yamlTag, "value", value, "fieldPath", fieldPath)
	}

	if foundValue != nil {
		// Set the field value
		err := y.setFieldValue(field, foundValue)
		if err != nil {
			y.debugLog("YamlFeeder: Failed to set field value", "fieldName", fieldName, "yamlKey", yamlTag, "value", foundValue, "error", err, "fieldPath", fieldPath)
			return err
		}

		// Record field population if tracker is available
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   foundKey,
				Value:       field.Interface(),
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    foundKey,
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}

		y.debugLog("YamlFeeder: Successfully set field value", "fieldName", fieldName, "yamlKey", yamlTag, "value", foundValue, "fieldPath", fieldPath)
	} else {
		// Record that we searched but didn't find
		if y.fieldTracker != nil {
			fp := FieldPopulation{
				FieldPath:   fieldPath,
				FieldName:   fieldName,
				FieldType:   field.Type().String(),
				FeederType:  "*feeders.YamlFeeder",
				SourceType:  "yaml",
				SourceKey:   "",
				Value:       nil,
				InstanceKey: "",
				SearchKeys:  searchKeys,
				FoundKey:    "",
			}
			y.fieldTracker.RecordFieldPopulation(fp)
		}

		y.debugLog("YamlFeeder: YAML value not found", "fieldName", fieldName, "yamlKey", yamlTag, "fieldPath", fieldPath)
	}

	return nil
}

// setMapFromYaml sets a map field value from YAML data with field tracking
func (y *YamlFeeder) setMapFromYaml(field reflect.Value, yamlData map[string]interface{}, fieldName, fieldPath string) error {
	if !field.CanSet() {
		return wrapYamlFieldCannotBeSetError()
	}

	mapType := field.Type()
	keyType := mapType.Key()
	valueType := mapType.Elem()

	y.debugLog("YamlFeeder: Setting map from YAML", "fieldName", fieldName, "mapType", mapType, "keyType", keyType, "valueType", valueType)

	// Create a new map
	newMap := reflect.MakeMap(mapType)

	// Handle different value types
	switch valueType.Kind() {
	case reflect.Struct:
		// Map of structs, like map[string]DBConnection
		for key, value := range yamlData {
			if valueMap, ok := value.(map[string]interface{}); ok {
				// Create a new instance of the struct type
				structValue := reflect.New(valueType).Elem()

				// Process the struct fields
				if err := y.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
					return fmt.Errorf("error processing map entry '%s': %w", key, err)
				}

				// Set the map entry
				keyValue := reflect.ValueOf(key)
				newMap.SetMapIndex(keyValue, structValue)
			} else {
				y.debugLog("YamlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
			}
		}
	case reflect.Ptr:
		// Map of pointers to structs, like map[string]*DBConnection
		elemType := valueType.Elem()
		if elemType.Kind() == reflect.Struct {
			for key, value := range yamlData {
				if valueMap, ok := value.(map[string]interface{}); ok {
					// Create a new instance of the struct type
					structValue := reflect.New(elemType).Elem()

					// Process the struct fields
					if err := y.processStructFields(structValue, valueMap, fieldPath+"."+key); err != nil {
						return fmt.Errorf("error processing map entry '%s': %w", key, err)
					}

					// Create a pointer to the struct and set the map entry
					ptrValue := reflect.New(elemType)
					ptrValue.Elem().Set(structValue)
					keyValue := reflect.ValueOf(key)
					newMap.SetMapIndex(keyValue, ptrValue)

					y.debugLog("YamlFeeder: Successfully processed pointer to struct map entry", "key", key, "structType", elemType)
				} else {
					y.debugLog("YamlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
				}
			}
		} else {
			// Map of pointers to non-struct types - handle direct conversion
			for key, value := range yamlData {
				keyValue := reflect.ValueOf(key)
				valueReflect := reflect.ValueOf(value)

				// Create a new pointer to the element type
				ptrValue := reflect.New(elemType)

				if valueReflect.Type().ConvertibleTo(elemType) {
					convertedValue := valueReflect.Convert(elemType)
					ptrValue.Elem().Set(convertedValue)
					newMap.SetMapIndex(keyValue, ptrValue)
				} else {
					y.debugLog("YamlFeeder: Cannot convert map value for pointer type", "key", key, "valueType", valueReflect.Type(), "targetType", elemType)
				}
			}
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.String,
		reflect.UnsafePointer:
		// Map of primitive types - use direct conversion
		for key, value := range yamlData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				y.debugLog("YamlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
			}
		}
	default:
		// Map of primitive types - use direct conversion
		for key, value := range yamlData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				y.debugLog("YamlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
			}
		}
	}

	// Set the field to the new map
	field.Set(newMap)

	// Record field population if tracker is available
	if y.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:   fieldPath,
			FieldName:   fieldName,
			FieldType:   field.Type().String(),
			FeederType:  "*feeders.YamlFeeder",
			SourceType:  "yaml",
			SourceKey:   fieldName, // For maps, use the field name as the source key
			Value:       field.Interface(),
			InstanceKey: "",
			SearchKeys:  []string{fieldName},
			FoundKey:    fieldName,
		}
		y.fieldTracker.RecordFieldPopulation(fp)
	}

	y.debugLog("YamlFeeder: Successfully set map field", "fieldName", fieldName, "mapSize", newMap.Len())

	return nil
}

// setFieldValue sets a reflect.Value from an interface{} value
func (y *YamlFeeder) setFieldValue(field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return wrapYamlFieldCannotBeSetError()
	}

	valueReflect := reflect.ValueOf(value)
	if !valueReflect.IsValid() {
		return nil // Skip nil values
	}

	// Special handling for time.Duration
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		if valueReflect.Kind() == reflect.String {
			str := valueReflect.String()
			duration, err := time.ParseDuration(str)
			if err != nil {
				return fmt.Errorf("cannot convert string '%s' to time.Duration: %w", str, err)
			}
			field.Set(reflect.ValueOf(duration))
			return nil
		}
		return wrapYamlTypeConversionError(valueReflect.Type().String(), field.Type().String())
	}

	// Handle type conversion
	if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
		return nil
	}

	// Handle slice types (like []string from []interface{})
	if field.Kind() == reflect.Slice && valueReflect.Kind() == reflect.Slice {
		return y.setSliceValue(field, valueReflect)
	}

	// Handle string conversion for basic types
	if valueReflect.Kind() == reflect.String {
		str := valueReflect.String()
		switch field.Kind() {
		case reflect.String:
			field.SetString(str)
		case reflect.Bool:
			switch str {
			case "true", "1":
				field.SetBool(true)
			case "false", "0":
				field.SetBool(false)
			default:
				return wrapYamlBoolConversionError(str)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if intVal, err := strconv.ParseInt(str, 10, 64); err == nil {
				field.SetInt(intVal)
			} else {
				return fmt.Errorf("cannot convert string '%s' to int: %w", str, err)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if uintVal, err := strconv.ParseUint(str, 10, 64); err == nil {
				field.SetUint(uintVal)
			} else {
				return fmt.Errorf("cannot convert string '%s' to uint: %w", str, err)
			}
		case reflect.Float32, reflect.Float64:
			if floatVal, err := strconv.ParseFloat(str, 64); err == nil {
				field.SetFloat(floatVal)
			} else {
				return fmt.Errorf("cannot convert string '%s' to float: %w", str, err)
			}
		case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
			reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
			reflect.Ptr, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
			return wrapYamlUnsupportedFieldTypeError(field.Type().String())
		default:
			return wrapYamlUnsupportedFieldTypeError(field.Type().String())
		}
		return nil
	}

	// Direct assignment for matching types
	if valueReflect.Type() == field.Type() {
		field.Set(valueReflect)
		return nil
	}

	return wrapYamlTypeConversionError(valueReflect.Type().String(), field.Type().String())
}

// setSliceValue converts []interface{} to specific slice types like []string
func (y *YamlFeeder) setSliceValue(field reflect.Value, valueReflect reflect.Value) error {
	if !field.CanSet() {
		return wrapYamlFieldCannotBeSetError()
	}

	sliceType := field.Type()
	elemType := sliceType.Elem()

	// Create a new slice of the correct type
	newSlice := reflect.MakeSlice(sliceType, valueReflect.Len(), valueReflect.Len())

	// Convert each element
	for i := 0; i < valueReflect.Len(); i++ {
		sourceElem := valueReflect.Index(i)
		targetElem := newSlice.Index(i)

		// Try direct conversion first
		if sourceElem.Type().ConvertibleTo(elemType) {
			targetElem.Set(sourceElem.Convert(elemType))
			continue
		}

		// Handle string conversion for basic types
		if sourceElem.Kind() == reflect.Interface {
			sourceElem = sourceElem.Elem() // Get the actual value from interface{}
		}

		if sourceElem.Type().ConvertibleTo(elemType) {
			targetElem.Set(sourceElem.Convert(elemType))
		} else if elemType.Kind() == reflect.String && sourceElem.CanInterface() {
			// Convert any value to string
			targetElem.SetString(fmt.Sprintf("%v", sourceElem.Interface()))
		} else {
			return wrapYamlTypeConversionError(sourceElem.Type().String(), elemType.String())
		}
	}

	field.Set(newSlice)
	return nil
}
