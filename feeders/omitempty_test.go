package feeders

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// OmitemptyTestConfig defines a structure with various omitempty tagged fields
type OmitemptyTestConfig struct {
	// Required fields (no omitempty)
	RequiredString string `yaml:"required_string" json:"required_string" toml:"required_string"`
	RequiredInt    int    `yaml:"required_int" json:"required_int" toml:"required_int"`

	// Optional fields with omitempty
	OptionalString  string  `yaml:"optional_string,omitempty" json:"optional_string,omitempty" toml:"optional_string,omitempty"`
	OptionalInt     int     `yaml:"optional_int,omitempty" json:"optional_int,omitempty" toml:"optional_int,omitempty"`
	OptionalBool    bool    `yaml:"optional_bool,omitempty" json:"optional_bool,omitempty" toml:"optional_bool,omitempty"`
	OptionalFloat64 float64 `yaml:"optional_float64,omitempty" json:"optional_float64,omitempty" toml:"optional_float64,omitempty"`

	// Pointer fields with omitempty
	OptionalStringPtr *string `yaml:"optional_string_ptr,omitempty" json:"optional_string_ptr,omitempty" toml:"optional_string_ptr,omitempty"`
	OptionalIntPtr    *int    `yaml:"optional_int_ptr,omitempty" json:"optional_int_ptr,omitempty" toml:"optional_int_ptr,omitempty"`

	// Slice fields with omitempty
	OptionalSlice []string `yaml:"optional_slice,omitempty" json:"optional_slice,omitempty" toml:"optional_slice,omitempty"`

	// Nested struct with omitempty
	OptionalNested *NestedConfig `yaml:"optional_nested,omitempty" json:"optional_nested,omitempty" toml:"optional_nested,omitempty"`
}

type NestedConfig struct {
	Name  string `yaml:"name" json:"name" toml:"name"`
	Value int    `yaml:"value" json:"value" toml:"value"`
}

func TestYAMLFeeder_OmitemptyHandling(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		expectFields map[string]interface{}
	}{
		{
			name: "all_fields_present",
			yamlContent: `
required_string: "test_string"
required_int: 42
optional_string: "optional_value"
optional_int: 123
optional_bool: true
optional_float64: 3.14
optional_string_ptr: "pointer_value"
optional_int_ptr: 456
optional_slice:
  - "item1"
  - "item2"
optional_nested:
  name: "nested_name"
  value: 789
`,
			expectFields: map[string]interface{}{
				"RequiredString":    "test_string",
				"RequiredInt":       42,
				"OptionalString":    "optional_value",
				"OptionalInt":       123,
				"OptionalBool":      true,
				"OptionalFloat64":   3.14,
				"OptionalStringPtr": "pointer_value",
				"OptionalIntPtr":    456,
				"OptionalSlice":     []string{"item1", "item2"},
				"OptionalNested":    &NestedConfig{Name: "nested_name", Value: 789},
			},
		},
		{
			name: "only_required_fields",
			yamlContent: `
required_string: "required_only"
required_int: 999
`,
			expectFields: map[string]interface{}{
				"RequiredString": "required_only",
				"RequiredInt":    999,
				// Optional fields should have zero values
				"OptionalString":    "",
				"OptionalInt":       0,
				"OptionalBool":      false,
				"OptionalFloat64":   0.0,
				"OptionalStringPtr": (*string)(nil),
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     ([]string)(nil),
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
		{
			name: "mixed_fields",
			yamlContent: `
required_string: "mixed_test"
required_int: 555
optional_string: "has_value"
optional_int: 777
# optional_bool is not provided
# optional_float64 is not provided
optional_string_ptr: "ptr_value"
# optional_int_ptr is not provided
optional_slice:
  - "single_item"
# optional_nested is not provided
`,
			expectFields: map[string]interface{}{
				"RequiredString":    "mixed_test",
				"RequiredInt":       555,
				"OptionalString":    "has_value",
				"OptionalInt":       777,
				"OptionalBool":      false, // zero value
				"OptionalFloat64":   0.0,   // zero value
				"OptionalStringPtr": "ptr_value",
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     []string{"single_item"},
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp YAML file
			tempFile, err := os.CreateTemp("", "test-omitempty-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("Failed to write YAML content: %v", err)
			}
			tempFile.Close()

			// Test YAML feeder
			feeder := NewYamlFeeder(tempFile.Name())
			var config OmitemptyTestConfig

			err = feeder.Feed(&config)
			if err != nil {
				t.Fatalf("YAML feeder failed: %v", err)
			}

			// Verify expected fields
			verifyOmitemptyTestConfig(t, "YAML", &config, tt.expectFields)
		})
	}
}

func TestTOMLFeeder_OmitemptyHandling(t *testing.T) {
	tests := []struct {
		name         string
		tomlContent  string
		expectFields map[string]interface{}
	}{
		{
			name: "all_fields_present",
			tomlContent: `
required_string = "test_string"
required_int = 42
optional_string = "optional_value"
optional_int = 123
optional_bool = true
optional_float64 = 3.14
optional_string_ptr = "pointer_value"
optional_int_ptr = 456
optional_slice = ["item1", "item2"]

[optional_nested]
name = "nested_name"
value = 789
`,
			expectFields: map[string]interface{}{
				"RequiredString":    "test_string",
				"RequiredInt":       42,
				"OptionalString":    "optional_value",
				"OptionalInt":       123,
				"OptionalBool":      true,
				"OptionalFloat64":   3.14,
				"OptionalStringPtr": "pointer_value",
				"OptionalIntPtr":    456,
				"OptionalSlice":     []string{"item1", "item2"},
				"OptionalNested":    &NestedConfig{Name: "nested_name", Value: 789},
			},
		},
		{
			name: "only_required_fields",
			tomlContent: `
required_string = "required_only"
required_int = 999
`,
			expectFields: map[string]interface{}{
				"RequiredString": "required_only",
				"RequiredInt":    999,
				// Optional fields should have zero values
				"OptionalString":    "",
				"OptionalInt":       0,
				"OptionalBool":      false,
				"OptionalFloat64":   0.0,
				"OptionalStringPtr": (*string)(nil),
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     ([]string)(nil),
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
		{
			name: "mixed_fields",
			tomlContent: `
required_string = "mixed_test"
required_int = 555
optional_string = "has_value"
optional_int = 777
optional_string_ptr = "ptr_value"
optional_slice = ["single_item"]
`,
			expectFields: map[string]interface{}{
				"RequiredString":    "mixed_test",
				"RequiredInt":       555,
				"OptionalString":    "has_value",
				"OptionalInt":       777,
				"OptionalBool":      false, // zero value
				"OptionalFloat64":   0.0,   // zero value
				"OptionalStringPtr": "ptr_value",
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     []string{"single_item"},
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp TOML file
			tempFile, err := os.CreateTemp("", "test-omitempty-*.toml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.tomlContent); err != nil {
				t.Fatalf("Failed to write TOML content: %v", err)
			}
			tempFile.Close()

			// Test TOML feeder
			feeder := NewTomlFeeder(tempFile.Name())
			var config OmitemptyTestConfig

			err = feeder.Feed(&config)
			if err != nil {
				t.Fatalf("TOML feeder failed: %v", err)
			}

			// Verify expected fields
			verifyOmitemptyTestConfig(t, "TOML", &config, tt.expectFields)
		})
	}
}

func TestJSONFeeder_OmitemptyHandling(t *testing.T) {
	tests := []struct {
		name         string
		jsonContent  string
		expectFields map[string]interface{}
	}{
		{
			name: "all_fields_present",
			jsonContent: `{
  "required_string": "test_string",
  "required_int": 42,
  "optional_string": "optional_value",
  "optional_int": 123,
  "optional_bool": true,
  "optional_float64": 3.14,
  "optional_string_ptr": "pointer_value",
  "optional_int_ptr": 456,
  "optional_slice": ["item1", "item2"],
  "optional_nested": {
    "name": "nested_name",
    "value": 789
  }
}`,
			expectFields: map[string]interface{}{
				"RequiredString":    "test_string",
				"RequiredInt":       42,
				"OptionalString":    "optional_value",
				"OptionalInt":       123,
				"OptionalBool":      true,
				"OptionalFloat64":   3.14,
				"OptionalStringPtr": "pointer_value",
				"OptionalIntPtr":    456,
				"OptionalSlice":     []string{"item1", "item2"},
				"OptionalNested":    &NestedConfig{Name: "nested_name", Value: 789},
			},
		},
		{
			name: "only_required_fields",
			jsonContent: `{
  "required_string": "required_only",
  "required_int": 999
}`,
			expectFields: map[string]interface{}{
				"RequiredString": "required_only",
				"RequiredInt":    999,
				// Optional fields should have zero values
				"OptionalString":    "",
				"OptionalInt":       0,
				"OptionalBool":      false,
				"OptionalFloat64":   0.0,
				"OptionalStringPtr": (*string)(nil),
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     ([]string)(nil),
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
		{
			name: "mixed_fields",
			jsonContent: `{
  "required_string": "mixed_test",
  "required_int": 555,
  "optional_string": "has_value",
  "optional_int": 777,
  "optional_string_ptr": "ptr_value",
  "optional_slice": ["single_item"]
}`,
			expectFields: map[string]interface{}{
				"RequiredString":    "mixed_test",
				"RequiredInt":       555,
				"OptionalString":    "has_value",
				"OptionalInt":       777,
				"OptionalBool":      false, // zero value
				"OptionalFloat64":   0.0,   // zero value
				"OptionalStringPtr": "ptr_value",
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     []string{"single_item"},
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
		{
			name: "null_values_in_json",
			jsonContent: `{
  "required_string": "null_test",
  "required_int": 111,
  "optional_string": "has_value",
  "optional_string_ptr": null,
  "optional_int_ptr": null,
  "optional_nested": null
}`,
			expectFields: map[string]interface{}{
				"RequiredString":    "null_test",
				"RequiredInt":       111,
				"OptionalString":    "has_value",
				"OptionalInt":       0,     // zero value
				"OptionalBool":      false, // zero value
				"OptionalFloat64":   0.0,   // zero value
				"OptionalStringPtr": (*string)(nil),
				"OptionalIntPtr":    (*int)(nil),
				"OptionalSlice":     ([]string)(nil),
				"OptionalNested":    (*NestedConfig)(nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp JSON file
			tempFile, err := os.CreateTemp("", "test-omitempty-*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.jsonContent); err != nil {
				t.Fatalf("Failed to write JSON content: %v", err)
			}
			tempFile.Close()

			// Test JSON feeder
			feeder := NewJSONFeeder(tempFile.Name())
			var config OmitemptyTestConfig

			err = feeder.Feed(&config)
			if err != nil {
				t.Fatalf("JSON feeder failed: %v", err)
			}

			// Verify expected fields
			verifyOmitemptyTestConfig(t, "JSON", &config, tt.expectFields)
		})
	}
}

// verifyOmitemptyTestConfig is a helper function to validate the populated config against expectations
func verifyOmitemptyTestConfig(t *testing.T, feederType string, config *OmitemptyTestConfig, expected map[string]interface{}) {
	t.Helper()

	// Check required fields
	if val, exists := expected["RequiredString"]; exists {
		if config.RequiredString != val.(string) {
			t.Errorf("[%s] RequiredString: expected %q, got %q", feederType, val.(string), config.RequiredString)
		}
	}

	if val, exists := expected["RequiredInt"]; exists {
		if config.RequiredInt != val.(int) {
			t.Errorf("[%s] RequiredInt: expected %d, got %d", feederType, val.(int), config.RequiredInt)
		}
	}

	// Check optional fields with omitempty
	if val, exists := expected["OptionalString"]; exists {
		if config.OptionalString != val.(string) {
			t.Errorf("[%s] OptionalString: expected %q, got %q", feederType, val.(string), config.OptionalString)
		}
	}

	if val, exists := expected["OptionalInt"]; exists {
		if config.OptionalInt != val.(int) {
			t.Errorf("[%s] OptionalInt: expected %d, got %d", feederType, val.(int), config.OptionalInt)
		}
	}

	if val, exists := expected["OptionalBool"]; exists {
		if config.OptionalBool != val.(bool) {
			t.Errorf("[%s] OptionalBool: expected %v, got %v", feederType, val.(bool), config.OptionalBool)
		}
	}

	if val, exists := expected["OptionalFloat64"]; exists {
		if config.OptionalFloat64 != val.(float64) {
			t.Errorf("[%s] OptionalFloat64: expected %f, got %f", feederType, val.(float64), config.OptionalFloat64)
		}
	}

	// Check pointer fields
	if val, exists := expected["OptionalStringPtr"]; exists {
		if val == nil {
			if config.OptionalStringPtr != nil {
				t.Errorf("[%s] OptionalStringPtr: expected nil, got %v", feederType, config.OptionalStringPtr)
			}
		} else {
			var expectedStr string
			switch v := val.(type) {
			case string:
				expectedStr = v
			case *string:
				if v == nil {
					if config.OptionalStringPtr != nil {
						t.Errorf("[%s] OptionalStringPtr: expected nil, got %v", feederType, config.OptionalStringPtr)
					}
					return
				}
				expectedStr = *v
			default:
				t.Errorf("[%s] OptionalStringPtr: unexpected type %T", feederType, val)
				return
			}
			if config.OptionalStringPtr == nil {
				t.Errorf("[%s] OptionalStringPtr: expected %q, got nil", feederType, expectedStr)
			} else if *config.OptionalStringPtr != expectedStr {
				t.Errorf("[%s] OptionalStringPtr: expected %q, got %q", feederType, expectedStr, *config.OptionalStringPtr)
			}
		}
	}

	if val, exists := expected["OptionalIntPtr"]; exists {
		if val == nil {
			if config.OptionalIntPtr != nil {
				t.Errorf("[%s] OptionalIntPtr: expected nil, got %v", feederType, config.OptionalIntPtr)
			}
		} else {
			var expectedInt int
			switch v := val.(type) {
			case int:
				expectedInt = v
			case *int:
				if v == nil {
					if config.OptionalIntPtr != nil {
						t.Errorf("[%s] OptionalIntPtr: expected nil, got %v", feederType, config.OptionalIntPtr)
					}
					return
				}
				expectedInt = *v
			default:
				t.Errorf("[%s] OptionalIntPtr: unexpected type %T", feederType, val)
				return
			}
			if config.OptionalIntPtr == nil {
				t.Errorf("[%s] OptionalIntPtr: expected %d, got nil", feederType, expectedInt)
			} else if *config.OptionalIntPtr != expectedInt {
				t.Errorf("[%s] OptionalIntPtr: expected %d, got %d", feederType, expectedInt, *config.OptionalIntPtr)
			}
		}
	}

	// Check slice field
	if val, exists := expected["OptionalSlice"]; exists {
		if val == nil {
			if config.OptionalSlice != nil {
				t.Errorf("[%s] OptionalSlice: expected nil, got %v", feederType, config.OptionalSlice)
			}
		} else {
			expectedSlice := val.([]string)
			if len(config.OptionalSlice) != len(expectedSlice) {
				t.Errorf("[%s] OptionalSlice: expected length %d, got length %d", feederType, len(expectedSlice), len(config.OptionalSlice))
			} else {
				for i, expected := range expectedSlice {
					if config.OptionalSlice[i] != expected {
						t.Errorf("[%s] OptionalSlice[%d]: expected %q, got %q", feederType, i, expected, config.OptionalSlice[i])
					}
				}
			}
		}
	}

	// Check nested struct field
	if val, exists := expected["OptionalNested"]; exists {
		if val == nil {
			if config.OptionalNested != nil {
				t.Errorf("[%s] OptionalNested: expected nil, got %v", feederType, config.OptionalNested)
			}
		} else {
			expectedNested := val.(*NestedConfig)
			if config.OptionalNested == nil {
				t.Errorf("[%s] OptionalNested: expected %+v, got nil", feederType, expectedNested)
			} else {
				if config.OptionalNested.Name != expectedNested.Name {
					t.Errorf("[%s] OptionalNested.Name: expected %q, got %q", feederType, expectedNested.Name, config.OptionalNested.Name)
				}
				if config.OptionalNested.Value != expectedNested.Value {
					t.Errorf("[%s] OptionalNested.Value: expected %d, got %d", feederType, expectedNested.Value, config.OptionalNested.Value)
				}
			}
		}
	}
}

// Test other tag modifiers besides omitempty
func TestTagModifiers_Comprehensive(t *testing.T) {
	type ConfigWithModifiers struct {
		// Different tag formats and modifiers
		FieldOmitempty    string `yaml:"field_omitempty,omitempty" json:"field_omitempty,omitempty" toml:"field_omitempty,omitempty"`
		FieldInline       string `yaml:",inline" json:",inline" toml:",inline"`
		FieldFlow         string `yaml:"field_flow,flow" json:"field_flow" toml:"field_flow"`
		FieldString       string `yaml:"field_string,string" json:"field_string,string" toml:"field_string"`
		FieldMultipleTags string `yaml:"field_multiple,omitempty,flow" json:"field_multiple,omitempty,string" toml:"field_multiple,omitempty"`
		FieldEmptyTagName string `yaml:",omitempty" json:",omitempty" toml:",omitempty"`
	}

	// Test with each feeder format
	testCases := []struct {
		name    string
		content string
		format  string
	}{
		{
			name: "yaml_with_modifiers",
			content: `
field_omitempty: "omitempty_value"
field_flow: "flow_value"
field_string: "string_value"
field_multiple: "multiple_value"
FieldEmptyTagName: "empty_tag_value"
`,
			format: "yaml",
		},
		{
			name: "json_with_modifiers",
			content: `{
  "field_omitempty": "omitempty_value",
  "field_flow": "flow_value", 
  "field_string": "string_value",
  "field_multiple": "multiple_value",
  "FieldEmptyTagName": "empty_tag_value"
}`,
			format: "json",
		},
		{
			name: "toml_with_modifiers",
			content: `
field_omitempty = "omitempty_value"
field_flow = "flow_value"
field_string = "string_value"
field_multiple = "multiple_value"
FieldEmptyTagName = "empty_tag_value"
`,
			format: "toml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp file
			tempFile, err := os.CreateTemp("", "test-modifiers-*."+tc.format)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tc.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}
			tempFile.Close()

			var config ConfigWithModifiers
			var feeder interface{ Feed(interface{}) error }

			// Create appropriate feeder
			switch tc.format {
			case "yaml":
				feeder = NewYamlFeeder(tempFile.Name())
			case "json":
				feeder = NewJSONFeeder(tempFile.Name())
			case "toml":
				feeder = NewTomlFeeder(tempFile.Name())
			default:
				t.Fatalf("Unknown format: %s", tc.format)
			}

			err = feeder.Feed(&config)
			if err != nil {
				t.Fatalf("%s feeder failed: %v", tc.format, err)
			}

			// Verify that values are properly set despite tag modifiers
			if config.FieldOmitempty != "omitempty_value" {
				t.Errorf("[%s] FieldOmitempty: expected 'omitempty_value', got '%s'", tc.format, config.FieldOmitempty)
			}
			if config.FieldFlow != "flow_value" {
				t.Errorf("[%s] FieldFlow: expected 'flow_value', got '%s'", tc.format, config.FieldFlow)
			}
			if config.FieldString != "string_value" {
				t.Errorf("[%s] FieldString: expected 'string_value', got '%s'", tc.format, config.FieldString)
			}
			if config.FieldMultipleTags != "multiple_value" {
				t.Errorf("[%s] FieldMultipleTags: expected 'multiple_value', got '%s'", tc.format, config.FieldMultipleTags)
			}
			if config.FieldEmptyTagName != "empty_tag_value" {
				t.Errorf("[%s] FieldEmptyTagName: expected 'empty_tag_value', got '%s'", tc.format, config.FieldEmptyTagName)
			}
		})
	}
}

// Test standard library behavior for comparison
func TestStandardLibraryBehavior(t *testing.T) {
	type StandardConfig struct {
		RequiredField string `yaml:"required" json:"required" toml:"required"`
		OptionalField string `yaml:"optional,omitempty" json:"optional,omitempty" toml:"optional,omitempty"`
	}

	testData := map[string]string{
		"yaml": `
required: "test_value"
optional: "optional_value"
`,
		"json": `{
  "required": "test_value",
  "optional": "optional_value"
}`,
		"toml": `
required = "test_value"
optional = "optional_value"
`,
	}

	for format, content := range testData {
		t.Run("stdlib_"+format, func(t *testing.T) {
			var config StandardConfig

			switch format {
			case "yaml":
				err := yaml.Unmarshal([]byte(content), &config)
				if err != nil {
					t.Fatalf("YAML unmarshal failed: %v", err)
				}
			case "json":
				err := json.Unmarshal([]byte(content), &config)
				if err != nil {
					t.Fatalf("JSON unmarshal failed: %v", err)
				}
			case "toml":
				err := toml.Unmarshal([]byte(content), &config)
				if err != nil {
					t.Fatalf("TOML unmarshal failed: %v", err)
				}
			}

			// Standard libraries should handle omitempty correctly
			if config.RequiredField != "test_value" {
				t.Errorf("[%s] RequiredField: expected 'test_value', got '%s'", format, config.RequiredField)
			}
			if config.OptionalField != "optional_value" {
				t.Errorf("[%s] OptionalField: expected 'optional_value', got '%s'", format, config.OptionalField)
			}
		})
	}
}
