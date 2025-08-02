package feeders

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

// ComprehensiveTypesConfig covers all major Go types for testing
type ComprehensiveTypesConfig struct {
	// Basic types
	StringField string `yaml:"stringField" json:"stringField" toml:"stringField"`
	BoolField   bool   `yaml:"boolField" json:"boolField" toml:"boolField"`

	// Integer types
	IntField    int    `yaml:"intField" json:"intField" toml:"intField"`
	Int8Field   int8   `yaml:"int8Field" json:"int8Field" toml:"int8Field"`
	Int16Field  int16  `yaml:"int16Field" json:"int16Field" toml:"int16Field"`
	Int32Field  int32  `yaml:"int32Field" json:"int32Field" toml:"int32Field"`
	Int64Field  int64  `yaml:"int64Field" json:"int64Field" toml:"int64Field"`
	UintField   uint   `yaml:"uintField" json:"uintField" toml:"uintField"`
	Uint8Field  uint8  `yaml:"uint8Field" json:"uint8Field" toml:"uint8Field"`
	Uint16Field uint16 `yaml:"uint16Field" json:"uint16Field" toml:"uint16Field"`
	Uint32Field uint32 `yaml:"uint32Field" json:"uint32Field" toml:"uint32Field"`
	Uint64Field uint64 `yaml:"uint64Field" json:"uint64Field" toml:"uint64Field"`

	// Floating point types
	Float32Field float32 `yaml:"float32Field" json:"float32Field" toml:"float32Field"`
	Float64Field float64 `yaml:"float64Field" json:"float64Field" toml:"float64Field"`

	// Pointer types
	StringPtr *string `yaml:"stringPtr" json:"stringPtr" toml:"stringPtr"`
	IntPtr    *int    `yaml:"intPtr" json:"intPtr" toml:"intPtr"`
	BoolPtr   *bool   `yaml:"boolPtr" json:"boolPtr" toml:"boolPtr"`

	// Slice types
	StringSlice []string            `yaml:"stringSlice" json:"stringSlice" toml:"stringSlice"`
	IntSlice    []int               `yaml:"intSlice" json:"intSlice" toml:"intSlice"`
	StructSlice []NestedTestStruct  `yaml:"structSlice" json:"structSlice" toml:"structSlice"`
	PtrSlice    []*NestedTestStruct `yaml:"ptrSlice" json:"ptrSlice" toml:"ptrSlice"`

	// Array types
	StringArray [3]string `yaml:"stringArray" json:"stringArray" toml:"stringArray"`
	IntArray    [2]int    `yaml:"intArray" json:"intArray" toml:"intArray"`

	// Map types
	StringMap    map[string]string            `yaml:"stringMap" json:"stringMap" toml:"stringMap"`
	IntMap       map[string]int               `yaml:"intMap" json:"intMap" toml:"intMap"`
	StructMap    map[string]NestedTestStruct  `yaml:"structMap" json:"structMap" toml:"structMap"`
	PtrStructMap map[string]*NestedTestStruct `yaml:"ptrStructMap" json:"ptrStructMap" toml:"ptrStructMap"`

	// Nested struct
	Nested NestedTestStruct `yaml:"nested" json:"nested" toml:"nested"`

	// Pointer to nested struct
	NestedPtr *NestedTestStruct `yaml:"nestedPtr" json:"nestedPtr" toml:"nestedPtr"`

	// Interface type (will be populated as interface{})
	InterfaceField interface{} `yaml:"interfaceField" json:"interfaceField" toml:"interfaceField"`

	// Custom type (type alias)
	CustomString CustomStringType `yaml:"customString" json:"customString" toml:"customString"`
	CustomInt    CustomIntType    `yaml:"customInt" json:"customInt" toml:"customInt"`
}

type NestedTestStruct struct {
	Name  string `yaml:"name" json:"name" toml:"name"`
	Value int    `yaml:"value" json:"value" toml:"value"`
}

type CustomStringType string
type CustomIntType int

// Test data generators
func createYAMLTestData() string {
	return `
stringField: "hello world"
boolField: true
intField: 42
int8Field: 127
int16Field: 32767
int32Field: 2147483647
int64Field: 9223372036854775807
uintField: 42
uint8Field: 255
uint16Field: 65535
uint32Field: 4294967295
uint64Field: 18446744073709551615
float32Field: 3.14159
float64Field: 2.718281828459045
stringPtr: "pointer string"
intPtr: 100
boolPtr: false
stringSlice:
  - "item1"
  - "item2"
  - "item3"
intSlice:
  - 1
  - 2
  - 3
structSlice:
  - name: "first"
    value: 10
  - name: "second"
    value: 20
ptrSlice:
  - name: "ptr1"
    value: 30
  - name: "ptr2"
    value: 40
stringArray:
  - "arr1"
  - "arr2"
  - "arr3"
intArray:
  - 100
  - 200
stringMap:
  key1: "value1"
  key2: "value2"
intMap:
  first: 1
  second: 2
structMap:
  item1:
    name: "struct1"
    value: 50
  item2:
    name: "struct2"
    value: 60
ptrStructMap:
  ptr1:
    name: "ptrStruct1"
    value: 70
  ptr2:
    name: "ptrStruct2"
    value: 80
nested:
  name: "nested struct"
  value: 999
nestedPtr:
  name: "nested pointer"
  value: 888
interfaceField: "interface value"
customString: "custom string value"
customInt: 12345
`
}

func createJSONTestData() string {
	return `{
	"stringField": "hello world",
	"boolField": true,
	"intField": 42,
	"int8Field": 127,
	"int16Field": 32767,
	"int32Field": 2147483647,
	"int64Field": 1234567890,
	"uintField": 42,
	"uint8Field": 255,
	"uint16Field": 65535,
	"uint32Field": 4294967295,
	"uint64Field": 1234567890,
	"float32Field": 3.14159,
	"float64Field": 2.718281828459045,
	"stringPtr": "pointer string",
	"intPtr": 100,
	"boolPtr": false,
	"stringSlice": ["item1", "item2", "item3"],
	"intSlice": [1, 2, 3],
	"structSlice": [
		{"name": "first", "value": 10},
		{"name": "second", "value": 20}
	],
	"ptrSlice": [
		{"name": "ptr1", "value": 30},
		{"name": "ptr2", "value": 40}
	],
	"stringArray": ["arr1", "arr2", "arr3"],
	"intArray": [100, 200],
	"stringMap": {
		"key1": "value1",
		"key2": "value2"
	},
	"intMap": {
		"first": 1,
		"second": 2
	},
	"structMap": {
		"item1": {"name": "struct1", "value": 50},
		"item2": {"name": "struct2", "value": 60}
	},
	"ptrStructMap": {
		"ptr1": {"name": "ptrStruct1", "value": 70},
		"ptr2": {"name": "ptrStruct2", "value": 80}
	},
	"nested": {
		"name": "nested struct",
		"value": 999
	},
	"nestedPtr": {
		"name": "nested pointer",
		"value": 888
	},
	"interfaceField": "interface value",
	"customString": "custom string value",
	"customInt": 12345
}`
}

func createTOMLTestData() string {
	// Note: TOML doesn't support complex numbers, and has issues with uint64 max values
	return `
stringField = "hello world"
boolField = true
intField = 42
int8Field = 127
int16Field = 32767
int32Field = 2147483647
int64Field = 9223372036854775807
uintField = 42
uint8Field = 255
uint16Field = 65535
uint32Field = 4294967295
uint64Field = 1844674407370955161
float32Field = 3.14159
float64Field = 2.718281828459045
stringPtr = "pointer string"
intPtr = 100
boolPtr = false
stringSlice = ["item1", "item2", "item3"]
intSlice = [1, 2, 3]
stringArray = ["arr1", "arr2", "arr3"]
intArray = [100, 200]
interfaceField = "interface value"
customString = "custom string value"
customInt = 12345

[[structSlice]]
name = "first"
value = 10

[[structSlice]]
name = "second"
value = 20

[[ptrSlice]]
name = "ptr1"
value = 30

[[ptrSlice]]
name = "ptr2"
value = 40

[stringMap]
key1 = "value1"
key2 = "value2"

[intMap]
first = 1
second = 2

[structMap.item1]
name = "struct1"
value = 50

[structMap.item2]
name = "struct2"
value = 60

[ptrStructMap.ptr1]
name = "ptrStruct1"
value = 70

[ptrStructMap.ptr2]
name = "ptrStruct2"
value = 80

[nested]
name = "nested struct"
value = 999

[nestedPtr]
name = "nested pointer"
value = 888
`
}

// Helper function to verify field tracking coverage
func verifyFieldTracking(t *testing.T, tracker *DefaultFieldTracker, feederType, sourceType string, expectedMinFields int) {
	populations := tracker.GetFieldPopulations()

	if len(populations) < expectedMinFields {
		t.Errorf("Expected at least %d field populations, got %d", expectedMinFields, len(populations))
	}

	// Track which fields we've seen
	fieldsSeen := make(map[string]bool)

	for _, pop := range populations {
		fieldsSeen[pop.FieldPath] = true

		// Verify basic tracking properties
		if pop.FeederType != feederType {
			t.Errorf("Expected FeederType '%s' for field %s, got '%s'", feederType, pop.FieldPath, pop.FeederType)
		}
		if pop.SourceType != sourceType {
			t.Errorf("Expected SourceType '%s' for field %s, got '%s'", sourceType, pop.FieldPath, pop.SourceType)
		}
		if pop.SourceKey == "" {
			t.Errorf("Expected non-empty SourceKey for field %s", pop.FieldPath)
		}
		if pop.FieldName == "" {
			t.Errorf("Expected non-empty FieldName for field %s", pop.FieldPath)
		}
		if pop.FieldType == "" {
			t.Errorf("Expected non-empty FieldType for field %s", pop.FieldPath)
		}
	}

	// Log field tracking for debugging
	t.Logf("Field tracking summary for %s:", feederType)
	for _, pop := range populations {
		t.Logf("  Field: %s (type: %s) = %v (from %s key: %s)",
			pop.FieldPath, pop.FieldType, pop.Value, pop.SourceType, pop.SourceKey)
	}
}

// Helper function to verify configuration values
func verifyComprehensiveConfigValues(t *testing.T, config *ComprehensiveTypesConfig, expectedUint64 uint64, expectedInt64 int64) {
	// Basic types
	if config.StringField != "hello world" {
		t.Errorf("Expected StringField 'hello world', got '%s'", config.StringField)
	}
	if !config.BoolField {
		t.Errorf("Expected BoolField true, got %v", config.BoolField)
	}

	// Integer types
	if config.IntField != 42 {
		t.Errorf("Expected IntField 42, got %d", config.IntField)
	}
	if config.Int8Field != 127 {
		t.Errorf("Expected Int8Field 127, got %d", config.Int8Field)
	}
	if config.Int16Field != 32767 {
		t.Errorf("Expected Int16Field 32767, got %d", config.Int16Field)
	}
	if config.Int32Field != 2147483647 {
		t.Errorf("Expected Int32Field 2147483647, got %d", config.Int32Field)
	}
	if config.Int64Field != expectedInt64 {
		t.Errorf("Expected Int64Field %d, got %d", expectedInt64, config.Int64Field)
	}
	if config.UintField != 42 {
		t.Errorf("Expected UintField 42, got %d", config.UintField)
	}
	if config.Uint8Field != 255 {
		t.Errorf("Expected Uint8Field 255, got %d", config.Uint8Field)
	}
	if config.Uint16Field != 65535 {
		t.Errorf("Expected Uint16Field 65535, got %d", config.Uint16Field)
	}
	if config.Uint32Field != 4294967295 {
		t.Errorf("Expected Uint32Field 4294967295, got %d", config.Uint32Field)
	}
	if config.Uint64Field != expectedUint64 {
		t.Errorf("Expected Uint64Field %d, got %d", expectedUint64, config.Uint64Field)
	}

	// Floating point types
	if fmt.Sprintf("%.5f", config.Float32Field) != "3.14159" {
		t.Errorf("Expected Float32Field 3.14159, got %f", config.Float32Field)
	}
	if fmt.Sprintf("%.15f", config.Float64Field) != "2.718281828459045" {
		t.Errorf("Expected Float64Field 2.718281828459045, got %f", config.Float64Field)
	}

	// Complex types were removed as they're not supported by the feeders

	// Pointer types
	if config.StringPtr == nil || *config.StringPtr != "pointer string" {
		t.Errorf("Expected StringPtr 'pointer string', got %v", config.StringPtr)
	}
	if config.IntPtr == nil || *config.IntPtr != 100 {
		t.Errorf("Expected IntPtr 100, got %v", config.IntPtr)
	}
	if config.BoolPtr == nil || *config.BoolPtr != false {
		t.Errorf("Expected BoolPtr false, got %v", config.BoolPtr)
	}

	// Slice types
	expectedStringSlice := []string{"item1", "item2", "item3"}
	if !reflect.DeepEqual(config.StringSlice, expectedStringSlice) {
		t.Errorf("Expected StringSlice %v, got %v", expectedStringSlice, config.StringSlice)
	}

	expectedIntSlice := []int{1, 2, 3}
	if !reflect.DeepEqual(config.IntSlice, expectedIntSlice) {
		t.Errorf("Expected IntSlice %v, got %v", expectedIntSlice, config.IntSlice)
	}

	if len(config.StructSlice) != 2 {
		t.Errorf("Expected StructSlice length 2, got %d", len(config.StructSlice))
	} else {
		if config.StructSlice[0].Name != "first" || config.StructSlice[0].Value != 10 {
			t.Errorf("Expected StructSlice[0] {first, 10}, got %+v", config.StructSlice[0])
		}
		if config.StructSlice[1].Name != "second" || config.StructSlice[1].Value != 20 {
			t.Errorf("Expected StructSlice[1] {second, 20}, got %+v", config.StructSlice[1])
		}
	}

	// Array types
	expectedStringArray := [3]string{"arr1", "arr2", "arr3"}
	if config.StringArray != expectedStringArray {
		t.Errorf("Expected StringArray %v, got %v", expectedStringArray, config.StringArray)
	}

	expectedIntArray := [2]int{100, 200}
	if config.IntArray != expectedIntArray {
		t.Errorf("Expected IntArray %v, got %v", expectedIntArray, config.IntArray)
	}

	// Map types
	if len(config.StringMap) != 2 || config.StringMap["key1"] != "value1" || config.StringMap["key2"] != "value2" {
		t.Errorf("Expected StringMap {key1:value1, key2:value2}, got %v", config.StringMap)
	}

	if len(config.IntMap) != 2 || config.IntMap["first"] != 1 || config.IntMap["second"] != 2 {
		t.Errorf("Expected IntMap {first:1, second:2}, got %v", config.IntMap)
	}

	// Nested struct
	if config.Nested.Name != "nested struct" || config.Nested.Value != 999 {
		t.Errorf("Expected Nested {nested struct, 999}, got %+v", config.Nested)
	}

	// Nested pointer
	if config.NestedPtr == nil || config.NestedPtr.Name != "nested pointer" || config.NestedPtr.Value != 888 {
		t.Errorf("Expected NestedPtr {nested pointer, 888}, got %+v", config.NestedPtr)
	}

	// Interface field
	if fmt.Sprintf("%v", config.InterfaceField) != "interface value" {
		t.Errorf("Expected InterfaceField 'interface value', got %v", config.InterfaceField)
	}

	// Custom types
	if config.CustomString != "custom string value" {
		t.Errorf("Expected CustomString 'custom string value', got '%s'", config.CustomString)
	}
	if config.CustomInt != 12345 {
		t.Errorf("Expected CustomInt 12345, got %d", config.CustomInt)
	}
}

func TestComprehensiveTypes_YAML(t *testing.T) {
	// Create test YAML file
	yamlContent := createYAMLTestData()

	tmpFile, err := os.CreateTemp("", "comprehensive_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test with field tracking enabled
	feeder := NewYamlFeeder(tmpFile.Name())
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	var config ComprehensiveTypesConfig
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed YAML config: %v", err)
	}

	// Verify all values are correct
	verifyComprehensiveConfigValues(t, &config, 18446744073709551615, 9223372036854775807)

	// Verify field tracking
	verifyFieldTracking(t, tracker, "*feeders.YamlFeeder", "yaml", 20)
}

func TestComprehensiveTypes_JSON(t *testing.T) {
	// Create test JSON file
	jsonContent := createJSONTestData()

	tmpFile, err := os.CreateTemp("", "comprehensive_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test with field tracking enabled
	feeder := NewJSONFeeder(tmpFile.Name())
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	var config ComprehensiveTypesConfig
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed JSON config: %v", err)
	}

	// Verify all values are correct
	verifyComprehensiveConfigValues(t, &config, 1234567890, 1234567890)

	// Verify field tracking
	verifyFieldTracking(t, tracker, "JSONFeeder", "json_file", 20)
}

func TestComprehensiveTypes_TOML(t *testing.T) {
	// Create test TOML file
	tomlContent := createTOMLTestData()

	tmpFile, err := os.CreateTemp("", "comprehensive_test_*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(tomlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test with field tracking enabled
	feeder := NewTomlFeeder(tmpFile.Name())
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	var config ComprehensiveTypesConfig
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed TOML config: %v", err)
	}

	// Verify all values are correct
	verifyComprehensiveConfigValues(t, &config, 1844674407370955161, 9223372036854775807)

	// Verify field tracking
	verifyFieldTracking(t, tracker, "TomlFeeder", "toml_file", 20)
}
