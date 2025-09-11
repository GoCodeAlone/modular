package modular

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
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
	t.Run("environment_variable_population_tracking", func(t *testing.T) {
		testEnvVariablePopulationTracking(t)
	})
}

// TestMixedYAMLAndEnvironmentPopulationTracking tests mixed YAML and environment tracking
// in a separate test function to ensure proper isolation
func TestMixedYAMLAndEnvironmentPopulationTracking(t *testing.T) {
	testMixedYAMLAndEnvironmentPopulationTracking(t)
}

func testEnvVariablePopulationTracking(t *testing.T) {
	envVars := map[string]string{
		"APP_NAME":            "Test App",
		"APP_DEBUG":           "true",
		"DB_PRIMARY_DRIVER":   "postgres",
		"DB_PRIMARY_DSN":      "postgres://localhost/primary",
		"DB_SECONDARY_DRIVER": "mysql",
		"DB_SECONDARY_DSN":    "mysql://localhost/secondary",
	}

	// Clean up any environment variables from previous tests - ensure complete isolation
	allTestEnvVars := []string{"APP_NAME", "APP_DEBUG", "DB_PRIMARY_DRIVER", "DB_PRIMARY_DSN", "DB_SECONDARY_DRIVER", "DB_SECONDARY_DSN"}
	for _, key := range allTestEnvVars {
		os.Unsetenv(key)
	}

	// Set up environment variables for this test
	for key, value := range envVars {
		os.Setenv(key, value)
	}

	// Immediate cleanup function (not deferred, executed at end of test)
	cleanup := func() {
		for _, key := range allTestEnvVars {
			os.Unsetenv(key)
		}
	}

	// Create logger that captures debug output
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create field tracker
	_ = NewConfigFieldTracker(mockLogger)

	// Create configuration structures with tracking
	type TestAppConfig struct {
		AppName string `env:"APP_NAME" yaml:"app_name"`
		Debug   bool   `env:"APP_DEBUG" yaml:"debug"`
	}

	appConfig := &TestAppConfig{}

	// Create field tracker
	tracker := NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Create configuration builder with field tracking
	cfg := NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	cfg.SetFieldTracker(tracker)

	// Add environment feeder (add last so it can override YAML)
	envFeeder := feeders.NewEnvFeeder()
	cfg.AddFeeder(envFeeder)

	// Add the configuration structure
	cfg.AddStructKey("app", appConfig)

	// Feed configuration
	err := cfg.Feed()
	require.NoError(t, err)

	// Verify that configuration was populated
	assert.Equal(t, "Test App", appConfig.AppName)
	assert.True(t, appConfig.Debug)

	// Verify field tracking captured the populations
	populations := tracker.FieldPopulations
	assert.GreaterOrEqual(t, len(populations), 2, "Should track at least 2 field populations")

	// Find specific field populations
	appNamePop := tracker.GetMostRelevantFieldPopulation("AppName")
	if assert.NotNil(t, appNamePop, "AppName field should be tracked") {
		assert.Equal(t, "Test App", appNamePop.Value)
		assert.Equal(t, "env", appNamePop.SourceType)
		assert.Equal(t, "APP_NAME", appNamePop.SourceKey)
	}

	debugPop := tracker.GetMostRelevantFieldPopulation("Debug")
	if assert.NotNil(t, debugPop, "Debug field should be tracked") {
		assert.Equal(t, true, debugPop.Value)
		assert.Equal(t, "env", debugPop.SourceType)
		assert.Equal(t, "APP_DEBUG", debugPop.SourceKey)
	}

	// Clean up for next test
	cleanup()
}

func testMixedYAMLAndEnvironmentPopulationTracking(t *testing.T) {
	// Use different env var names to avoid conflicts with other tests
	envVars := map[string]string{
		"MIXED_APP_NAME":          "Test App",
		"MIXED_DB_PRIMARY_DRIVER": "postgres",
		"MIXED_DB_PRIMARY_DSN":    "postgres://localhost/primary",
	}
	yamlData := `
debug: false
connections:
  secondary:
    driver: "mysql"
    dsn: "mysql://localhost/secondary"
`

	// Clean up any environment variables from previous tests - ensure complete isolation
	allTestEnvVars := []string{"MIXED_APP_NAME", "MIXED_APP_DEBUG", "MIXED_DB_PRIMARY_DRIVER", "MIXED_DB_PRIMARY_DSN", "MIXED_DB_SECONDARY_DRIVER", "MIXED_DB_SECONDARY_DSN"}
	for _, key := range allTestEnvVars {
		os.Unsetenv(key)
	}

	// Set up environment variables for this test
	for key, value := range envVars {
		os.Setenv(key, value)
	}

	// Immediate cleanup function (not deferred, executed at end of test)
	cleanup := func() {
		for _, key := range allTestEnvVars {
			os.Unsetenv(key)
		}
	}

	// Create logger that captures debug output
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create field tracker
	_ = NewConfigFieldTracker(mockLogger)

	// Create configuration structures with tracking - use MIXED env var names
	type TestAppConfig struct {
		AppName string `env:"MIXED_APP_NAME" yaml:"app_name"`
		Debug   bool   `env:"MIXED_APP_DEBUG" yaml:"debug"`
	}

	appConfig := &TestAppConfig{}

	// Create field tracker
	tracker := NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Create configuration builder with field tracking
	cfg := NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	cfg.SetFieldTracker(tracker)

	// Add YAML feeder if test includes YAML data (add first so env can override)
	// Create temporary YAML file
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_%s.yaml", strings.ReplaceAll("mixed", " ", "_")))
	err := os.WriteFile(tempFile, []byte(yamlData), 0644)
	require.NoError(t, err)
	defer os.Remove(tempFile)

	// Add YAML feeder
	yamlFeeder := feeders.NewYamlFeeder(tempFile)
	cfg.AddFeeder(yamlFeeder)

	// Add environment feeder (add last so it can override YAML)
	envFeeder := feeders.NewEnvFeeder()
	cfg.AddFeeder(envFeeder)

	// Add the configuration structure
	cfg.AddStructKey("app", appConfig)

	// Feed configuration
	err = cfg.Feed()
	require.NoError(t, err)

	// Verify mixed YAML and environment population
	assert.Equal(t, "Test App", appConfig.AppName)
	assert.False(t, appConfig.Debug) // debug should be false from YAML, not true from env

	// Verify field tracking captured the populations from both sources
	populations := tracker.FieldPopulations
	assert.GreaterOrEqual(t, len(populations), 2, "Should track at least 2 field populations")

	// Find specific field populations
	appNamePop := tracker.GetMostRelevantFieldPopulation("AppName")
	if assert.NotNil(t, appNamePop, "AppName field should be tracked") {
		assert.Equal(t, "Test App", appNamePop.Value)
		assert.Equal(t, "env", appNamePop.SourceType)
		assert.Equal(t, "MIXED_APP_NAME", appNamePop.SourceKey)
	}

	debugPop := tracker.GetMostRelevantFieldPopulation("Debug")
	if assert.NotNil(t, debugPop, "Debug field should be tracked") {
		assert.Equal(t, false, debugPop.Value)
		assert.Equal(t, "yaml", debugPop.SourceType)
		assert.Equal(t, "debug", debugPop.SourceKey)
	}

	// Clean up for next test
	cleanup()
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

	// This test verifies that we can track exactly:
	// 1. Which environment variables are searched for each field
	// 2. Which ones are found vs not found
	// 3. Which feeder populated each field
	// 4. The exact source key that was used
	// 5. The instance key for instance-aware fields

	// Define the database configuration structure that supports instance-aware configuration
	type DBConnection struct {
		Driver   string `env:"DRIVER" desc:"Database driver"`
		DSN      string `env:"DSN" desc:"Database connection string"`
		MaxConns int    `env:"MAX_CONNS" desc:"Maximum connections"`
	}

	type DBConfig struct {
		Connections map[string]*DBConnection
	}

	// Create configuration instance with pre-initialized instances
	dbConfig := &DBConfig{
		Connections: map[string]*DBConnection{
			"primary": {
				Driver:   "default_driver",
				DSN:      "default_dsn",
				MaxConns: 1,
			},
			"secondary": {
				Driver:   "default_driver",
				DSN:      "default_dsn",
				MaxConns: 1,
			},
		},
	}

	// Create field tracker
	tracker := NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Add instance-aware environment feeder
	instanceFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + strings.ToUpper(instanceKey) + "_"
	})

	// Set up field tracking bridge
	bridge := NewFieldTrackingBridge(tracker)
	instanceFeeder.SetFieldTracker(bridge)

	// Use FeedInstances to populate the instance-aware configuration
	err := instanceFeeder.FeedInstances(dbConfig.Connections)
	require.NoError(t, err)

	// Verify that configuration was populated correctly
	assert.Contains(t, dbConfig.Connections, "primary")
	assert.Contains(t, dbConfig.Connections, "secondary")

	if primary, ok := dbConfig.Connections["primary"]; ok {
		assert.Equal(t, "postgres", primary.Driver)
		assert.Equal(t, "postgres://user:pass@localhost:5432/primary_db", primary.DSN)
		assert.Equal(t, 10, primary.MaxConns)
	}

	if secondary, ok := dbConfig.Connections["secondary"]; ok {
		assert.Equal(t, "mysql", secondary.Driver)
		assert.Equal(t, "mysql://user:pass@localhost:3306/secondary_db", secondary.DSN)
		assert.Equal(t, 5, secondary.MaxConns)
	}

	// Verify field tracking captured the instance-aware populations
	populations := tracker.FieldPopulations
	assert.GreaterOrEqual(t, len(populations), 6, "Should track at least 6 field populations (3 fields × 2 instances)")

	// Since the field paths are just the field names (Driver, DSN, MaxConns), let's verify by instance key
	// Find all populations for primary instance
	var primaryDriverPop, primaryDSNPop, primaryMaxConnsPop *FieldPopulation
	var secondaryDriverPop, secondaryDSNPop, secondaryMaxConnsPop *FieldPopulation

	for _, fp := range tracker.FieldPopulations {
		if fp.InstanceKey == "primary" {
			switch fp.FieldName {
			case "Driver":
				primaryDriverPop = &fp
			case "DSN":
				primaryDSNPop = &fp
			case "MaxConns":
				primaryMaxConnsPop = &fp
			}
		} else if fp.InstanceKey == "secondary" {
			switch fp.FieldName {
			case "Driver":
				secondaryDriverPop = &fp
			case "DSN":
				secondaryDSNPop = &fp
			case "MaxConns":
				secondaryMaxConnsPop = &fp
			}
		}
	}

	// Verify primary instance fields
	if assert.NotNil(t, primaryDriverPop, "Primary Driver should be tracked") {
		assert.Equal(t, "DB_PRIMARY_DRIVER", primaryDriverPop.SourceKey)
		assert.Equal(t, "postgres", primaryDriverPop.Value)
		assert.Equal(t, "primary", primaryDriverPop.InstanceKey)
	}

	if assert.NotNil(t, primaryDSNPop, "Primary DSN should be tracked") {
		assert.Equal(t, "DB_PRIMARY_DSN", primaryDSNPop.SourceKey)
		assert.Equal(t, "postgres://user:pass@localhost:5432/primary_db", primaryDSNPop.Value)
		assert.Equal(t, "primary", primaryDSNPop.InstanceKey)
	}

	if assert.NotNil(t, primaryMaxConnsPop, "Primary MaxConns should be tracked") {
		assert.Equal(t, "DB_PRIMARY_MAX_CONNS", primaryMaxConnsPop.SourceKey)
		assert.Equal(t, 10, primaryMaxConnsPop.Value)
		assert.Equal(t, "primary", primaryMaxConnsPop.InstanceKey)
	}

	// Verify secondary instance fields
	if assert.NotNil(t, secondaryDriverPop, "Secondary Driver should be tracked") {
		assert.Equal(t, "DB_SECONDARY_DRIVER", secondaryDriverPop.SourceKey)
		assert.Equal(t, "mysql", secondaryDriverPop.Value)
		assert.Equal(t, "secondary", secondaryDriverPop.InstanceKey)
	}

	if assert.NotNil(t, secondaryDSNPop, "Secondary DSN should be tracked") {
		assert.Equal(t, "DB_SECONDARY_DSN", secondaryDSNPop.SourceKey)
		assert.Equal(t, "mysql://user:pass@localhost:3306/secondary_db", secondaryDSNPop.Value)
		assert.Equal(t, "secondary", secondaryDSNPop.InstanceKey)
	}

	if assert.NotNil(t, secondaryMaxConnsPop, "Secondary MaxConns should be tracked") {
		assert.Equal(t, "DB_SECONDARY_MAX_CONNS", secondaryMaxConnsPop.SourceKey)
		assert.Equal(t, 5, secondaryMaxConnsPop.Value)
		assert.Equal(t, "secondary", secondaryMaxConnsPop.InstanceKey)
	}
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

			config := &TestConfig{}

			// Create mock logger for capturing debug output
			mockLogger := new(MockLogger)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

			// Create field tracker
			tracker := NewDefaultFieldTracker()
			tracker.SetLogger(mockLogger)

			// Create struct state differ for tracking field changes
			differ := NewStructStateDiffer(tracker, mockLogger)

			// Capture before state
			differ.CaptureBeforeState(config, "")

			// Create and apply environment feeder
			envFeeder := feeders.NewEnvFeeder()
			err := envFeeder.Feed(config)
			require.NoError(t, err)

			// Capture after state and compute diffs
			differ.CaptureAfterStateAndDiff(config, "", "*feeders.EnvFeeder", "env")

			// Verify that the expected field changes were detected
			for fieldPath, expectedValue := range tt.expectedFieldDiffs {
				pop := tracker.GetMostRelevantFieldPopulation(fieldPath)
				if assert.NotNil(t, pop, "Field %s should be tracked via diff", fieldPath) {
					assert.Equal(t, expectedValue, pop.Value, "Field %s should have expected value", fieldPath)
					assert.Equal(t, "*feeders.EnvFeeder", pop.FeederType, "Field %s should be tracked as EnvFeeder", fieldPath)
					assert.Equal(t, "env", pop.SourceType, "Field %s should have env source type", fieldPath)
					assert.Equal(t, "detected_by_diff", pop.SourceKey, "Field %s should be marked as detected by diff", fieldPath)
				}
			}
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

	config := &TestConfig{}
	mockLogger := new(MockLogger)

	// Capture all debug log calls
	var debugCalls [][]interface{}
	mockLogger.On("Debug", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		debugCalls = append(debugCalls, args)
	}).Return()

	// Create environment feeder with verbose debug enabled
	envFeeder := feeders.NewEnvFeeder()
	envFeeder.SetVerboseDebug(true, mockLogger)

	// Feed the configuration - this should generate debug logs
	err := envFeeder.Feed(config)
	require.NoError(t, err)

	// Verify the configuration was populated correctly
	assert.Equal(t, "test_value", config.TestField)

	// Verify that verbose debug logging includes the required information
	requiredLogMessages := []string{
		"Processing field", // Field name being processed
		"Looking up env",   // Environment variable search
		"Found env",        // Environment variable found
		"Successfully set", // Field population success
	}

	for _, requiredMsg := range requiredLogMessages {
		found := false
		for _, call := range debugCalls {
			if len(call) > 0 {
				if msg, ok := call[0].(string); ok && strings.Contains(msg, requiredMsg) {
					found = true
					break
				}
			}
		}
		assert.True(t, found, "Required debug message not found: %s", requiredMsg)
	}
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
