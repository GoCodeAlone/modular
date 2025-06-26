package feeders

import (
	"fmt"
	"os"
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
// of the same configuration type using instance-specific prefixes
type InstanceAwareEnvFeeder struct {
	prefixFunc InstancePrefixFunc
}

// Ensure InstanceAwareEnvFeeder implements both interfaces
var _ interface {
	Feed(interface{}) error
	FeedKey(string, interface{}) error
	FeedInstances(interface{}) error
} = (*InstanceAwareEnvFeeder)(nil)

// NewInstanceAwareEnvFeeder creates a new instance-aware environment variable feeder
func NewInstanceAwareEnvFeeder(prefixFunc InstancePrefixFunc) *InstanceAwareEnvFeeder {
	return &InstanceAwareEnvFeeder{
		prefixFunc: prefixFunc,
	}
}

// Feed implements the basic Feeder interface for single instances (backward compatibility)
func (f *InstanceAwareEnvFeeder) Feed(structure interface{}) error {
	inputType := reflect.TypeOf(structure)
	if inputType == nil {
		return ErrEnvInvalidStructure
	}

	if inputType.Kind() != reflect.Ptr {
		return ErrEnvInvalidStructure
	}

	if inputType.Elem().Kind() != reflect.Struct {
		return ErrEnvInvalidStructure
	}

	// For single instance, use no prefix
	return f.feedStructWithPrefix(reflect.ValueOf(structure).Elem(), "")
}

// FeedKey implements the ComplexFeeder interface for instance-specific feeding
func (f *InstanceAwareEnvFeeder) FeedKey(instanceKey string, structure interface{}) error {
	inputType := reflect.TypeOf(structure)
	if inputType == nil {
		return ErrEnvInvalidStructure
	}

	if inputType.Kind() != reflect.Ptr {
		return ErrEnvInvalidStructure
	}

	if inputType.Elem().Kind() != reflect.Struct {
		return ErrEnvInvalidStructure
	}

	// Generate prefix for this instance
	prefix := ""
	if f.prefixFunc != nil {
		prefix = f.prefixFunc(instanceKey)
	}

	return f.feedStructWithPrefix(reflect.ValueOf(structure).Elem(), prefix)
}

// FeedInstances feeds multiple instances of the same configuration type
func (f *InstanceAwareEnvFeeder) FeedInstances(instances interface{}) error {
	instancesValue := reflect.ValueOf(instances)
	if instancesValue.Kind() != reflect.Map {
		return ErrInstancesMustBeMap
	}

	// Iterate through map entries
	for _, key := range instancesValue.MapKeys() {
		instanceKey := key.String()
		instance := instancesValue.MapIndex(key)

		// Create a pointer to the instance for modification
		instancePtr := reflect.New(instance.Type())
		instancePtr.Elem().Set(instance)

		// Feed this instance with its specific prefix
		if err := f.FeedKey(instanceKey, instancePtr.Interface()); err != nil {
			return fmt.Errorf("failed to feed instance '%s': %w", instanceKey, err)
		}

		// Update the map with the modified instance
		instancesValue.SetMapIndex(key, instancePtr.Elem())
	}

	return nil
}

// feedStructWithPrefix feeds a struct with environment variables using the specified prefix
func (f *InstanceAwareEnvFeeder) feedStructWithPrefix(rv reflect.Value, prefix string) error {
	return f.processStructFieldsWithPrefix(rv, prefix)
}

// processStructFieldsWithPrefix iterates through struct fields with prefix
func (f *InstanceAwareEnvFeeder) processStructFieldsWithPrefix(rv reflect.Value, prefix string) error {
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rv.Type().Field(i)

		if err := f.processFieldWithPrefix(field, &fieldType, prefix); err != nil {
			return fmt.Errorf("error in field '%s': %w", fieldType.Name, err)
		}
	}
	return nil
}

// processFieldWithPrefix handles a single struct field with prefix
func (f *InstanceAwareEnvFeeder) processFieldWithPrefix(field reflect.Value, fieldType *reflect.StructField, prefix string) error {
	// Handle nested structs
	switch field.Kind() {
	case reflect.Struct:
		return f.processStructFieldsWithPrefix(field, prefix)
	case reflect.Pointer:
		if !field.IsZero() && field.Elem().Kind() == reflect.Struct {
			return f.processStructFieldsWithPrefix(field.Elem(), prefix)
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Map, reflect.Slice, reflect.String, reflect.UnsafePointer:
		// Check for env tag for primitive types and other non-struct types
		if envTag, exists := fieldType.Tag.Lookup("env"); exists {
			return f.setFieldFromEnvWithPrefix(field, envTag, prefix)
		}
	}

	return nil
}

// setFieldFromEnvWithPrefix sets a field value from an environment variable with prefix
func (f *InstanceAwareEnvFeeder) setFieldFromEnvWithPrefix(field reflect.Value, envTag, prefix string) error {
	// Build environment variable name with prefix
	envName := strings.ToUpper(envTag)
	if prefix != "" {
		envName = strings.ToUpper(prefix) + envName
	}

	// Get and apply environment variable if exists
	if envValue := os.Getenv(envName); envValue != "" {
		return setFieldValue(field, envValue)
	}
	return nil
}
