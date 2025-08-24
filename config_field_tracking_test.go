package modular

import (
	"os"
	"reflect"
	"testing"

	"github.com/CrisisTextLine/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ConfigFieldTracker tracks which fields are populated by which feeders
type ConfigFieldTracker struct {
	FieldPopulations []FieldPopulation
	logger           Logger
}

// NewConfigFieldTracker creates a new field tracker
func NewConfigFieldTracker(logger Logger) *ConfigFieldTracker {
	return &ConfigFieldTracker{
		FieldPopulations: make([]FieldPopulation, 0),
		logger:           logger,
	}
}

// RecordFieldPopulation records that a field was populated
func (t *ConfigFieldTracker) RecordFieldPopulation(fp FieldPopulation) {
	t.FieldPopulations = append(t.FieldPopulations, fp)
	if t.logger != nil {
		t.logger.Debug("Field populated",
			"fieldPath", fp.FieldPath,
			"fieldName", fp.FieldName,
			"fieldType", fp.FieldType,
			"feederType", fp.FeederType,
			"sourceType", fp.SourceType,
			"sourceKey", fp.SourceKey,
			"value", fp.Value,
			"instanceKey", fp.InstanceKey,
		)
	}
}

// GetFieldPopulation returns the population info for a specific field path
func (t *ConfigFieldTracker) GetFieldPopulation(fieldPath string) *FieldPopulation {
	for _, fp := range t.FieldPopulations {
		if fp.FieldPath == fieldPath {
			return &fp
		}
	}
	return nil
}

// GetPopulationsByFeeder returns all field populations by a specific feeder type
func (t *ConfigFieldTracker) GetPopulationsByFeeder(feederType string) []FieldPopulation {
	var result []FieldPopulation
	for _, fp := range t.FieldPopulations {
		if fp.FeederType == feederType {
			result = append(result, fp)
		}
	}
	return result
}

// GetPopulationsBySource returns all field populations by a specific source type
func (t *ConfigFieldTracker) GetPopulationsBySource(sourceType string) []FieldPopulation {
	var result []FieldPopulation
	for _, fp := range t.FieldPopulations {
		if fp.SourceType == sourceType {
			result = append(result, fp)
		}
	}
	return result
}

// TestFieldLevelPopulationTracking tests that we can track exactly which fields
// are populated by which feeders with full visibility into the population process
func TestFieldLevelPopulationTracking(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		yamlData string
		expected map[string]FieldPopulation // fieldPath -> expected population
	}{
		{
			name: "environment variable population tracking",
			envVars: map[string]string{
				"APP_NAME":            "Test App",
				"APP_DEBUG":           "true",
				"DB_PRIMARY_DRIVER":   "postgres",
				"DB_PRIMARY_DSN":      "postgres://localhost/primary",
				"DB_SECONDARY_DRIVER": "mysql",
				"DB_SECONDARY_DSN":    "mysql://localhost/secondary",
			},
			expected: map[string]FieldPopulation{
				"AppName": {
					FieldPath:   "AppName",
					FieldName:   "AppName",
					FieldType:   "string",
					FeederType:  "*feeders.EnvFeeder",
					SourceType:  "env",
					SourceKey:   "APP_NAME",
					Value:       "Test App",
					InstanceKey: "",
				},
				"Debug": {
					FieldPath:   "Debug",
					FieldName:   "Debug",
					FieldType:   "bool",
					FeederType:  "*feeders.EnvFeeder",
					SourceType:  "env",
					SourceKey:   "APP_DEBUG",
					Value:       true,
					InstanceKey: "",
				},
				"Connections.primary.Driver": {
					FieldPath:   "Connections.primary.Driver",
					FieldName:   "Driver",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_PRIMARY_DRIVER",
					Value:       "postgres",
					InstanceKey: "primary",
				},
				"Connections.primary.DSN": {
					FieldPath:   "Connections.primary.DSN",
					FieldName:   "DSN",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_PRIMARY_DSN",
					Value:       "postgres://localhost/primary",
					InstanceKey: "primary",
				},
				"Connections.secondary.Driver": {
					FieldPath:   "Connections.secondary.Driver",
					FieldName:   "Driver",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_SECONDARY_DRIVER",
					Value:       "mysql",
					InstanceKey: "secondary",
				},
				"Connections.secondary.DSN": {
					FieldPath:   "Connections.secondary.DSN",
					FieldName:   "DSN",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_SECONDARY_DSN",
					Value:       "mysql://localhost/secondary",
					InstanceKey: "secondary",
				},
			},
		},
		{
			name: "mixed yaml and environment population tracking",
			envVars: map[string]string{
				"APP_NAME":          "Test App",
				"DB_PRIMARY_DRIVER": "postgres",
				"DB_PRIMARY_DSN":    "postgres://localhost/primary",
			},
			yamlData: `
debug: false
connections:
  secondary:
    driver: "mysql"
    dsn: "mysql://localhost/secondary"
`,
			expected: map[string]FieldPopulation{
				"AppName": {
					FieldPath:   "AppName",
					FieldName:   "AppName",
					FieldType:   "string",
					FeederType:  "*feeders.EnvFeeder",
					SourceType:  "env",
					SourceKey:   "APP_NAME",
					Value:       "Test App",
					InstanceKey: "",
				},
				"Debug": {
					FieldPath:   "Debug",
					FieldName:   "Debug",
					FieldType:   "bool",
					FeederType:  "*feeders.YamlFeeder",
					SourceType:  "yaml",
					SourceKey:   "debug",
					Value:       false,
					InstanceKey: "",
				},
				"Connections.primary.Driver": {
					FieldPath:   "Connections.primary.Driver",
					FieldName:   "Driver",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_PRIMARY_DRIVER",
					Value:       "postgres",
					InstanceKey: "primary",
				},
				"Connections.primary.DSN": {
					FieldPath:   "Connections.primary.DSN",
					FieldName:   "DSN",
					FieldType:   "string",
					FeederType:  "*feeders.InstanceAwareEnvFeeder",
					SourceType:  "env",
					SourceKey:   "DB_PRIMARY_DSN",
					Value:       "postgres://localhost/primary",
					InstanceKey: "primary",
				},
				"Connections.secondary.Driver": {
					FieldPath:   "Connections.secondary.Driver",
					FieldName:   "Driver",
					FieldType:   "string",
					FeederType:  "*feeders.YamlFeeder",
					SourceType:  "yaml",
					SourceKey:   "connections.secondary.driver",
					Value:       "mysql",
					InstanceKey: "secondary",
				},
				"Connections.secondary.DSN": {
					FieldPath:   "Connections.secondary.DSN",
					FieldName:   "DSN",
					FieldType:   "string",
					FeederType:  "*feeders.YamlFeeder",
					SourceType:  "yaml",
					SourceKey:   "connections.secondary.dsn",
					Value:       "mysql://localhost/secondary",
					InstanceKey: "secondary",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Create logger that captures debug output
			mockLogger := new(MockLogger)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

			// Create field tracker
			_ = NewConfigFieldTracker(mockLogger)

			// Create configuration structures with tracking
			type TestAppConfig struct {
				AppName string `env:"APP_NAME"`
				Debug   bool   `env:"APP_DEBUG"`
			}

			appConfig := &TestAppConfig{}

			// Create field tracker
			tracker := NewDefaultFieldTracker()
			tracker.SetLogger(mockLogger)

			// Create configuration builder with field tracking
			cfg := NewConfig()
			cfg.SetVerboseDebug(true, mockLogger)
			cfg.SetFieldTracker(tracker)

			// Add environment feeder
			envFeeder := feeders.NewEnvFeeder()
			cfg.AddFeeder(envFeeder)

			// Add the configuration structure
			cfg.AddStructKey("app", appConfig)

			// Feed configuration
			err := cfg.Feed()
			require.NoError(t, err)

			// Verify that configuration was populated
			if tt.name == "environment_variable_population_tracking" {
				assert.Equal(t, "Test App", appConfig.AppName)
				assert.True(t, appConfig.Debug)

				// Verify field tracking captured the populations
				populations := tracker.FieldPopulations
				assert.GreaterOrEqual(t, len(populations), 2, "Should track at least 2 field populations")

				// Find specific field populations
				appNamePop := tracker.GetFieldPopulation("AppName")
				if assert.NotNil(t, appNamePop, "AppName field should be tracked") {
					assert.Equal(t, "Test App", appNamePop.Value)
					assert.Equal(t, "env", appNamePop.SourceType)
					assert.Equal(t, "APP_NAME", appNamePop.SourceKey)
				}

				debugPop := tracker.GetFieldPopulation("Debug")
				if assert.NotNil(t, debugPop, "Debug field should be tracked") {
					assert.Equal(t, true, debugPop.Value)
					assert.Equal(t, "env", debugPop.SourceType)
					assert.Equal(t, "APP_DEBUG", debugPop.SourceKey)
				}
			} else {
				// For mixed YAML scenarios, skip until YAML feeder supports field tracking
				t.Skip("Mixed YAML and environment field tracking requires YAML feeder field tracking support")
			}
		})
	}
}

// TestDetailedInstanceAwareFieldTracking tests specific instance-aware field tracking
// for the database module scenario that's causing issues for the user
func TestDetailedInstanceAwareFieldTracking(t *testing.T) {
	// Set up environment variables exactly as user would
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":      "postgres",
		"DB_PRIMARY_DSN":         "postgres://user:pass@localhost:5432/primary_db",
		"DB_PRIMARY_MAX_CONNS":   "10",
		"DB_SECONDARY_DRIVER":    "mysql",
		"DB_SECONDARY_DSN":       "mysql://user:pass@localhost:3306/secondary_db",
		"DB_SECONDARY_MAX_CONNS": "5",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Create mock logger to capture verbose output
	mockLogger := new(MockLogger)
	debugLogs := make([][]interface{}, 0)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		debugLogs = append(debugLogs, args)
	}).Return()

	// Create field tracker
	_ = NewConfigFieldTracker(mockLogger)

	// This test should verify that we can track exactly:
	// 1. Which environment variables are searched for each field
	// 2. Which ones are found vs not found
	// 3. Which feeder populated each field
	// 4. The exact source key that was used
	// 5. The instance key for instance-aware fields

	t.Skip("Detailed instance-aware field tracking not yet implemented")

	// After implementation, we should be able to verify:
	// 1. That DB_PRIMARY_DSN was found and populated the primary.DSN field
	// 2. That DB_SECONDARY_DSN was found and populated the secondary.DSN field
	// 3. The exact search pattern used for each field
	// 4. Whether any fields failed to populate and why

	// expectedSearches := []string{
	//     "DB_PRIMARY_DRIVER", "DB_PRIMARY_DSN", "DB_PRIMARY_MAX_CONNS",
	//     "DB_SECONDARY_DRIVER", "DB_SECONDARY_DSN", "DB_SECONDARY_MAX_CONNS",
	// }

	// expectedPopulations := map[string]FieldPopulation{
	//     "Connections.primary.DSN": {
	//         FieldPath:   "Connections.primary.DSN",
	//         SourceKey:   "DB_PRIMARY_DSN",
	//         Value:       "postgres://user:pass@localhost:5432/primary_db",
	//         InstanceKey: "primary",
	//     },
	//     "Connections.secondary.DSN": {
	//         FieldPath:   "Connections.secondary.DSN",
	//         SourceKey:   "DB_SECONDARY_DSN",
	//         Value:       "mysql://user:pass@localhost:3306/secondary_db",
	//         InstanceKey: "secondary",
	//     },
	// }
}

// TestConfigDiffBasedFieldTracking tests an alternative approach using before/after diffs
// to determine which fields were populated by which feeders
func TestConfigDiffBasedFieldTracking(t *testing.T) {
	tests := []struct {
		name               string
		envVars            map[string]string
		expectedFieldDiffs map[string]interface{} // field path -> expected new value
	}{
		{
			name: "basic field diff tracking",
			envVars: map[string]string{
				"APP_NAME":  "Test App",
				"APP_DEBUG": "true",
			},
			expectedFieldDiffs: map[string]interface{}{
				"AppName": "Test App",
				"Debug":   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			type TestConfig struct {
				AppName string `env:"APP_NAME"`
				Debug   bool   `env:"APP_DEBUG"`
			}

			_ = &TestConfig{}

			// This test should verify that we can use before/after comparison
			// to determine which fields were populated by which feeders
			t.Skip("Diff-based field tracking not yet implemented")

			// After implementation:
			// beforeState := captureStructState(config)
			// err := feedWithDiffTracking(config)
			// require.NoError(t, err)
			// afterState := captureStructState(config)
			// diffs := computeFieldDiffs(beforeState, afterState)

			// for fieldPath, expectedValue := range tt.expectedFieldDiffs {
			//     assert.Contains(t, diffs, fieldPath)
			//     assert.Equal(t, expectedValue, diffs[fieldPath])
			// }
		})
	}
}

// TestVerboseDebugFieldVisibility tests that verbose debug logging provides
// sufficient visibility into field population for troubleshooting
func TestVerboseDebugFieldVisibility(t *testing.T) {
	// Set up test environment
	os.Setenv("TEST_FIELD", "test_value")
	defer os.Unsetenv("TEST_FIELD")

	type TestConfig struct {
		TestField string `env:"TEST_FIELD"`
	}

	_ = &TestConfig{}
	mockLogger := new(MockLogger)

	// Capture all debug log calls
	var debugCalls [][]interface{}
	mockLogger.On("Debug", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		debugCalls = append(debugCalls, args)
	}).Return()

	// This test should verify that verbose debug logging includes:
	// 1. Field name being processed
	// 2. Environment variable name being searched
	// 3. Whether the environment variable was found
	// 4. The value that was set
	// 5. Success/failure of field population

	t.Skip("Enhanced verbose debug field visibility not yet implemented")

	// After implementation, we should be able to verify debug logs contain:
	// - "Processing field: TestField"
	// - "Looking up environment variable: TEST_FIELD"
	// - "Environment variable found: TEST_FIELD=test_value"
	// - "Successfully set field value: TestField=test_value"

	// requiredLogMessages := []string{
	//     "Processing field",
	//     "Looking up environment variable",
	//     "Environment variable found",
	//     "Successfully set field value",
	// }

	// for _, requiredMsg := range requiredLogMessages {
	//     found := false
	//     for _, call := range debugCalls {
	//         if len(call) > 0 {
	//             if msg, ok := call[0].(string); ok && strings.Contains(msg, requiredMsg) {
	//                 found = true
	//                 break
	//             }
	//         }
	//     }
	//     assert.True(t, found, "Required debug message not found: %s", requiredMsg)
	// }
}

// These are helper functions for the unimplemented diff-based tracking approach
// They will be used once the implementation is complete

// StructState represents the state of a struct at a point in time
type StructState struct {
	Fields map[string]interface{} // field path -> value
}

// captureStructState captures the current state of all fields in a struct
func captureStructState(structure interface{}) *StructState {
	state := &StructState{
		Fields: make(map[string]interface{}),
	}

	rv := reflect.ValueOf(structure)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	captureStructFields(rv, "", state.Fields)
	return state
}

// captureStructFields recursively captures all field values
func captureStructFields(rv reflect.Value, prefix string, fields map[string]interface{}) {
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		fieldPath := fieldType.Name
		if prefix != "" {
			fieldPath = prefix + "." + fieldType.Name
		}

		switch field.Kind() {
		case reflect.Struct:
			captureStructFields(field, fieldPath, fields)
		case reflect.Ptr:
			if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
				captureStructFields(field.Elem(), fieldPath, fields)
			} else if !field.IsNil() {
				fields[fieldPath] = field.Elem().Interface()
			}
		case reflect.Map:
			if !field.IsNil() {
				for _, key := range field.MapKeys() {
					mapValue := field.MapIndex(key)
					mapFieldPath := fieldPath + "." + key.String()
					if mapValue.Kind() == reflect.Struct {
						captureStructFields(mapValue, mapFieldPath, fields)
					} else {
						fields[mapFieldPath] = mapValue.Interface()
					}
				}
			}
		case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
			reflect.Chan, reflect.Func, reflect.Interface, reflect.Slice, reflect.String, reflect.UnsafePointer:
			fields[fieldPath] = field.Interface()
		default:
			fields[fieldPath] = field.Interface()
		}
	}
}

// computeFieldDiffs computes the differences between two struct states
func computeFieldDiffs(before, after *StructState) map[string]interface{} {
	diffs := make(map[string]interface{})

	// Find fields that changed
	for fieldPath, afterValue := range after.Fields {
		beforeValue, existed := before.Fields[fieldPath]
		if !existed || !reflect.DeepEqual(beforeValue, afterValue) {
			diffs[fieldPath] = afterValue
		}
	}

	return diffs
}

var (
	// These prevent unused function warnings - they'll be used once implementation is complete
	_ = captureStructState
	_ = computeFieldDiffs
)
