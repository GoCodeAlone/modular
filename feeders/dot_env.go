package feeders

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// DotEnvFeeder is a feeder that reads .env files and populates configuration directly from the parsed values
type DotEnvFeeder struct {
	Path         string
	verboseDebug bool
	logger       interface {
		Debug(msg string, args ...any)
	}
	fieldTracker FieldTracker
	envVars      map[string]string // in-memory storage of parsed .env variables
}

// NewDotEnvFeeder creates a new DotEnvFeeder that reads from the specified .env file
func NewDotEnvFeeder(filePath string) *DotEnvFeeder {
	return &DotEnvFeeder{
		Path:         filePath,
		verboseDebug: false,
		logger:       nil,
		fieldTracker: nil,
		envVars:      make(map[string]string),
	}
}

// SetVerboseDebug enables or disables verbose debug logging
func (f *DotEnvFeeder) SetVerboseDebug(enabled bool, logger interface{ Debug(msg string, args ...any) }) {
	f.verboseDebug = enabled
	f.logger = logger
	if enabled && logger != nil {
		f.logger.Debug("Verbose dot env feeder debugging enabled")
	}
}

// SetFieldTracker sets the field tracker for recording field populations
func (f *DotEnvFeeder) SetFieldTracker(tracker FieldTracker) {
	f.fieldTracker = tracker
}

// Feed reads the .env file and populates the provided structure directly
func (f *DotEnvFeeder) Feed(structure interface{}) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Starting feed process", "filePath", f.Path, "structureType", reflect.TypeOf(structure))
	}

	// Parse the .env file into memory first (for tracking purposes)
	err := f.parseDotEnvFile()
	if err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Failed to parse .env file", "filePath", f.Path, "error", err)
		}
		return fmt.Errorf("failed to parse .env file: %w", err)
	}

	// Load into global environment catalog for other env feeders to use
	catalog := GetGlobalEnvCatalog()
	catalogErr := catalog.LoadFromDotEnv(f.Path)
	if catalogErr != nil && f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Failed to load into global catalog", "error", catalogErr)
		// Don't fail the operation if catalog loading fails
	}

	// Populate the structure from the global catalog (respects OS env precedence)
	return f.populateStructFromCatalog(structure, "")
}

// parseDotEnvFile parses the .env file into the envVars map
func (f *DotEnvFeeder) parseDotEnvFile() error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Parsing .env file", "filePath", f.Path)
	}

	// Clear existing parsed values
	f.envVars = make(map[string]string)

	file, err := os.Open(f.Path)
	if err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Failed to open .env file", "filePath", f.Path, "error", err)
		}
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("DotEnvFeeder: Skipping line", "lineNum", lineNum, "reason", "empty or comment")
			}
			continue
		}

		// Parse key=value pairs
		if err := f.parseEnvLine(line, lineNum); err != nil {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("DotEnvFeeder: Failed to parse line", "lineNum", lineNum, "line", line, "error", err)
			}
			return fmt.Errorf("failed to parse .env line: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		if f.verboseDebug && f.logger != nil {
			f.logger.Debug("DotEnvFeeder: Scanner error", "error", err)
		}
		return fmt.Errorf("scanner error: %w", err)
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Successfully parsed .env file", "filePath", f.Path, "linesProcessed", lineNum, "varsFound", len(f.envVars))
	}
	return nil
}

// parseEnvLine parses a single line from the .env file and stores it in memory
func (f *DotEnvFeeder) parseEnvLine(line string, lineNum int) error {
	// Find the first = character
	idx := strings.Index(line, "=")
	if idx == -1 {
		return fmt.Errorf("%w at line %d: %s", ErrDotEnvInvalidLineFormat, lineNum, line)
	}

	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])

	// Remove quotes if present
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Parsed variable", "key", key, "value", value, "lineNum", lineNum)
	}

	// Store the variable in memory (do NOT set in environment)
	f.envVars[key] = value
	return nil
}

// populateStructFromCatalog populates struct fields from the global environment catalog
func (f *DotEnvFeeder) populateStructFromCatalog(structure interface{}, prefix string) error {
	structValue := reflect.ValueOf(structure)
	if structValue.Kind() != reflect.Ptr || structValue.Elem().Kind() != reflect.Struct {
		return wrapDotEnvStructureError(structure)
	}

	return f.processStructFieldsFromCatalog(structValue.Elem(), prefix)
}

// processStructFieldsFromCatalog iterates through struct fields and populates them from the global catalog
func (f *DotEnvFeeder) processStructFieldsFromCatalog(rv reflect.Value, prefix string) error {
	structType := rv.Type()
	catalog := GetGlobalEnvCatalog()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" || envTag == "-" {
			// Handle nested structs
			if field.Kind() == reflect.Struct {
				// For DotEnv, we don't use prefixes since env tags should be complete
				err := f.processStructFieldsFromCatalog(field, prefix)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Build the field path for tracking
		fieldPath := fieldType.Name
		if prefix != "" {
			fieldPath = prefix + "." + fieldPath
		}

		// For DotEnv, use the env tag directly as it should contain the complete variable name
		envKey := envTag

		// Get value from catalog
		value, exists := catalog.Get(envKey)
		if !exists || value == "" {
			if f.verboseDebug && f.logger != nil {
				f.logger.Debug("DotEnvFeeder: Environment variable not found", "envKey", envKey, "fieldPath", fieldPath)
			}
			continue
		}

		if f.verboseDebug && f.logger != nil {
			source := catalog.GetSource(envKey)
			f.logger.Debug("DotEnvFeeder: Setting field from catalog", "envKey", envKey, "value", value, "fieldPath", fieldPath, "source", source)
		}

		// Set the field value
		err := f.setFieldValue(field, fieldType, value, fieldPath, envKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// setFieldValue sets a field value from .env data with type conversion
func (f *DotEnvFeeder) setFieldValue(field reflect.Value, fieldType reflect.StructField, value, fieldPath, envKey string) error {
	if f.verboseDebug && f.logger != nil {
		f.logger.Debug("DotEnvFeeder: Setting field", "fieldPath", fieldPath, "envKey", envKey, "value", value, "fieldType", field.Type())
	}

	// Convert the string value to the appropriate type
	convertedValue, err := f.convertStringToType(value, field.Type())
	if err != nil {
		return fmt.Errorf("failed to convert value '%s' for field %s: %w", value, fieldPath, err)
	}

	// Set the field value
	field.Set(reflect.ValueOf(convertedValue))

	// Record field population
	if f.fieldTracker != nil {
		fp := FieldPopulation{
			FieldPath:  fieldPath,
			FieldName:  fieldType.Name,
			FieldType:  field.Type().String(),
			FeederType: "DotEnvFeeder",
			SourceType: "dot_env_file",
			SourceKey:  envKey,
			Value:      convertedValue,
			SearchKeys: []string{envKey},
			FoundKey:   envKey,
		}
		f.fieldTracker.RecordFieldPopulation(fp)
	}

	return nil
}

// convertStringToType converts a string value to the target type
func (f *DotEnvFeeder) convertStringToType(value string, targetType reflect.Type) (interface{}, error) {
	switch targetType.Kind() {
	case reflect.String:
		return value, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot convert '%s' to %s: %w", value, targetType.Kind(), err)
		}
		return reflect.ValueOf(intVal).Convert(targetType).Interface(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot convert '%s' to %s: %w", value, targetType.Kind(), err)
		}
		return reflect.ValueOf(uintVal).Convert(targetType).Interface(), nil
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot convert '%s' to %s: %w", value, targetType.Kind(), err)
		}
		return reflect.ValueOf(floatVal).Convert(targetType).Interface(), nil
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("cannot convert '%s' to bool: %w", value, err)
		}
		return boolVal, nil
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Ptr, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return nil, wrapDotEnvUnsupportedTypeError(targetType.Kind().String())
	default:
		return nil, wrapDotEnvUnsupportedTypeError(targetType.Kind().String())
	}
}
