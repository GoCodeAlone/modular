package feeders

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// YamlFeeder is a feeder that reads YAML files with optional verbose debug logging
type YamlFeeder struct {
	Path         string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
}

// NewYamlFeeder creates a new YamlFeeder that reads from the specified YAML file
func NewYamlFeeder(filePath string) *YamlFeeder {
	return &YamlFeeder{
		Path:         filePath,
		verboseDebug: false,
		logger:       nil,
		fieldTracker: nil,
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (y *YamlFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	y.verboseDebug = enabled
	y.logger = logger
	if enabled && logger != nil {
		y.logger.Debug("Verbose YAML feeder debugging enabled")
	}
}

// SetFieldTracker sets the field tracker for recording field populations
func (y *YamlFeeder) SetFieldTracker(tracker FieldTracker) {
	y.fieldTracker = tracker
}

// Feed reads the YAML file and populates the provided structure
func (y *YamlFeeder) Feed(structure interface{}) error {
	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Starting feed process", "filePath", y.Path, "structureType", reflect.TypeOf(structure))
	}

	// Always use custom parsing logic for consistency
	err := y.feedWithTracking(structure)

	if y.verboseDebug && y.logger != nil {
		if err != nil {
			y.logger.Debug("YamlFeeder: Feed completed with error", "filePath", y.Path, "error", err)
		} else {
			y.logger.Debug("YamlFeeder: Feed completed successfully", "filePath", y.Path)
		}
	}
	if err != nil {
		return fmt.Errorf("yaml feed error: %w", err)
	}
	return nil
}

// FeedKey reads a YAML file and extracts a specific key
func (y *YamlFeeder) FeedKey(key string, target interface{}) error {
	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Starting FeedKey process", "filePath", y.Path, "key", key, "targetType", reflect.TypeOf(target))
	}

	// Create a temporary map to hold all YAML data
	var allData map[interface{}]interface{}

	// Use the embedded Yaml feeder to read the file
	if err := y.Feed(&allData); err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to read YAML file", "filePath", y.Path, "error", err)
		}
		return fmt.Errorf("failed to read YAML: %w", err)
	}

	// Look for the specific key
	value, exists := allData[key]
	if !exists {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Key not found in YAML file", "filePath", y.Path, "key", key)
		}
		return nil
	}

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Found key in YAML file", "filePath", y.Path, "key", key, "valueType", reflect.TypeOf(value))
	}

	// Remarshal and unmarshal to handle type conversions
	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to marshal value", "filePath", y.Path, "key", key, "error", err)
		}
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err = yaml.Unmarshal(valueBytes, target); err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to unmarshal value to target", "filePath", y.Path, "key", key, "error", err)
		}
		return fmt.Errorf("failed to unmarshal value to target: %w", err)
	}

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: FeedKey completed successfully", "filePath", y.Path, "key", key)
	}
	return nil
}

// feedWithTracking processes YAML data with field tracking support
func (y *YamlFeeder) feedWithTracking(structure interface{}) error {
	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Starting feedWithTracking", "filePath", y.Path, "structureType", reflect.TypeOf(structure))
	}

	// Read YAML file
	content, err := os.ReadFile(y.Path)
	if err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to read YAML file", "filePath", y.Path, "error", err)
		}
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	// Check if we're dealing with a struct pointer
	structValue := reflect.ValueOf(structure)
	if structValue.Kind() != reflect.Ptr || structValue.Elem().Kind() != reflect.Struct {
		// Not a struct pointer, fall back to standard YAML unmarshaling
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Not a struct pointer, using standard YAML unmarshaling", "structureType", reflect.TypeOf(structure))
		}
		if err := yaml.Unmarshal(content, structure); err != nil {
			return fmt.Errorf("failed to unmarshal YAML data: %w", err)
		}
		return nil
	}

	// Parse YAML content
	data := make(map[string]interface{})
	if err := yaml.Unmarshal(content, &data); err != nil {
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Failed to parse YAML content", "filePath", y.Path, "error", err)
		}
		return fmt.Errorf("failed to parse YAML content: %w", err)
	}

	// Process the structure fields with tracking
	return y.processStructFields(reflect.ValueOf(structure).Elem(), data, "")
}

// processStructFields processes struct fields and tracks field populations from YAML data
func (y *YamlFeeder) processStructFields(rv reflect.Value, data map[string]interface{}, parentPath string) error {
	structType := rv.Type()

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Processing struct fields", "structType", structType, "numFields", rv.NumField(), "parentPath", parentPath)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Build field path
		fieldPath := fieldType.Name
		if parentPath != "" {
			fieldPath = parentPath + "." + fieldType.Name
		}

		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Processing field", "fieldName", fieldType.Name, "fieldType", fieldType.Type, "fieldPath", fieldPath)
		}

		if err := y.processField(field, &fieldType, data, fieldPath); err != nil {
			if y.verboseDebug && y.logger != nil {
				y.logger.Debug("YamlFeeder: Field processing failed", "fieldName", fieldType.Name, "error", err)
			}
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}
	}
	return nil
}

// processField handles a single struct field with YAML data and field tracking
func (y *YamlFeeder) processField(field reflect.Value, fieldType *reflect.StructField, data map[string]interface{}, fieldPath string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Ptr:
		// Handle pointer types
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			return y.setPointerFromYAML(field, yamlTag, data, fieldType.Name, fieldPath)
		}
	case reflect.Slice:
		// Handle slice types
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			return y.setSliceFromYAML(field, yamlTag, data, fieldType.Name, fieldPath)
		}
	case reflect.Array:
		// Handle array types
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			return y.setArrayFromYAML(field, yamlTag, data, fieldType.Name, fieldPath)
		}
	case reflect.Map:
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Processing map field", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}

		// Check if there's a yaml tag for this map
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			// Look for map data using the yaml tag
			if mapData, found := data[yamlTag]; found {
				if mapDataTyped, ok := mapData.(map[string]interface{}); ok {
					return y.setMapFromYaml(field, mapDataTyped, fieldType.Name, fieldPath)
				} else {
					if y.verboseDebug && y.logger != nil {
						y.logger.Debug("YamlFeeder: Map YAML data is not a map[string]interface{}", "fieldName", fieldType.Name, "yamlTag", yamlTag, "dataType", reflect.TypeOf(mapData))
					}
				}
			} else {
				if y.verboseDebug && y.logger != nil {
					y.logger.Debug("YamlFeeder: Map YAML data not found", "fieldName", fieldType.Name, "yamlTag", yamlTag)
				}
			}
		}
	case reflect.Struct:
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Processing nested struct", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}

		// Check if there's a yaml tag for this nested struct
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			// Look for nested data using the yaml tag
			if nestedData, found := data[yamlTag]; found {
				if nestedMap, ok := nestedData.(map[string]interface{}); ok {
					return y.processStructFields(field, nestedMap, fieldPath)
				} else {
					if y.verboseDebug && y.logger != nil {
						y.logger.Debug("YamlFeeder: Nested YAML data is not a map", "fieldName", fieldType.Name, "yamlTag", yamlTag, "dataType", reflect.TypeOf(nestedData))
					}
				}
			} else {
				if y.verboseDebug && y.logger != nil {
					y.logger.Debug("YamlFeeder: Nested YAML data not found", "fieldName", fieldType.Name, "yamlTag", yamlTag)
				}
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
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			if y.verboseDebug && y.logger != nil {
				y.logger.Debug("YamlFeeder: Found yaml tag", "fieldName", fieldType.Name, "yamlTag", yamlTag, "fieldPath", fieldPath)
			}
			return y.setFieldFromYaml(field, yamlTag, data, fieldType.Name, fieldPath)
		} else if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: No yaml tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}
	default:
		// Check for yaml tag for primitive types and other non-struct types
		if yamlTag, exists := fieldType.Tag.Lookup("yaml"); exists {
			if y.verboseDebug && y.logger != nil {
				y.logger.Debug("YamlFeeder: Found yaml tag", "fieldName", fieldType.Name, "yamlTag", yamlTag, "fieldPath", fieldPath)
			}
			return y.setFieldFromYaml(field, yamlTag, data, fieldType.Name, fieldPath)
		} else if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: No yaml tag found", "fieldName", fieldType.Name, "fieldPath", fieldPath)
		}
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
		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Found YAML value", "fieldName", fieldName, "yamlKey", yamlTag, "value", value, "fieldPath", fieldPath)
		}
	}

	if foundValue != nil {
		// Set the field value
		err := y.setFieldValue(field, foundValue)
		if err != nil {
			if y.verboseDebug && y.logger != nil {
				y.logger.Debug("YamlFeeder: Failed to set field value", "fieldName", fieldName, "yamlKey", yamlTag, "value", foundValue, "error", err, "fieldPath", fieldPath)
			}
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

		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: Successfully set field value", "fieldName", fieldName, "yamlKey", yamlTag, "value", foundValue, "fieldPath", fieldPath)
		}
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

		if y.verboseDebug && y.logger != nil {
			y.logger.Debug("YamlFeeder: YAML value not found", "fieldName", fieldName, "yamlKey", yamlTag, "fieldPath", fieldPath)
		}
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

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Setting map from YAML", "fieldName", fieldName, "mapType", mapType, "keyType", keyType, "valueType", valueType)
	}

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
				if y.verboseDebug && y.logger != nil {
					y.logger.Debug("YamlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
				}
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

					if y.verboseDebug && y.logger != nil {
						y.logger.Debug("YamlFeeder: Successfully processed pointer to struct map entry", "key", key, "structType", elemType)
					}
				} else {
					if y.verboseDebug && y.logger != nil {
						y.logger.Debug("YamlFeeder: Map entry is not a map", "key", key, "valueType", reflect.TypeOf(value))
					}
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
					if y.verboseDebug && y.logger != nil {
						y.logger.Debug("YamlFeeder: Cannot convert map value for pointer type", "key", key, "valueType", valueReflect.Type(), "targetType", elemType)
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
		for key, value := range yamlData {
			keyValue := reflect.ValueOf(key)
			valueReflect := reflect.ValueOf(value)

			if valueReflect.Type().ConvertibleTo(valueType) {
				convertedValue := valueReflect.Convert(valueType)
				newMap.SetMapIndex(keyValue, convertedValue)
			} else {
				if y.verboseDebug && y.logger != nil {
					y.logger.Debug("YamlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
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
				if y.verboseDebug && y.logger != nil {
					y.logger.Debug("YamlFeeder: Cannot convert map value", "key", key, "valueType", valueReflect.Type(), "targetType", valueType)
				}
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

	if y.verboseDebug && y.logger != nil {
		y.logger.Debug("YamlFeeder: Successfully set map field", "fieldName", fieldName, "mapSize", newMap.Len())
	}

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
