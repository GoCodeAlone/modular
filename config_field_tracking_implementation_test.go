package modular

import (
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// FieldTrackingBridge bridges between the main package FieldTracker interface
// and the feeders package FieldTracker interface to avoid circular imports
type FieldTrackingBridge struct {
	mainTracker FieldTracker
}

// NewFieldTrackingBridge creates a bridge between the interfaces
func NewFieldTrackingBridge(tracker FieldTracker) *FieldTrackingBridge {
	return &FieldTrackingBridge{mainTracker: tracker}
}

// RecordFieldPopulation bridges field population records
func (b *FieldTrackingBridge) RecordFieldPopulation(fp feeders.FieldPopulation) {
	// Convert from feeders.FieldPopulation to main.FieldPopulation
	mainFP := FieldPopulation{
		FieldPath:   fp.FieldPath,
		FieldName:   fp.FieldName,
		FieldType:   fp.FieldType,
		FeederType:  fp.FeederType,
		SourceType:  fp.SourceType,
		SourceKey:   fp.SourceKey,
		Value:       fp.Value,
		InstanceKey: fp.InstanceKey,
		SearchKeys:  fp.SearchKeys,
		FoundKey:    fp.FoundKey,
	}
	b.mainTracker.RecordFieldPopulation(mainFP)
}

// SetupFieldTrackingForFeeders sets up field tracking for feeders that support it
func SetupFieldTrackingForFeeders(cfgFeeders []Feeder, tracker FieldTracker) {
	bridge := NewFieldTrackingBridge(tracker)

	for _, feeder := range cfgFeeders {
		// Use type assertion for the specific feeder types we know about
		switch f := feeder.(type) {
		case interface{ SetFieldTracker(feeders.FieldTracker) }:
			f.SetFieldTracker(bridge)
		}
	}
}

// TestEnhancedFieldTracking tests the enhanced field tracking functionality
func TestEnhancedFieldTracking(t *testing.T) {
	// NOTE: This test uses t.Setenv, so it must NOT call t.Parallel on the same *testing.T
	// per Go 1.25 rules (tests using t.Setenv or t.Chdir cannot use t.Parallel). Keep it
	// serial to avoid panic: "test using t.Setenv or t.Chdir can not use t.Parallel".
	tests := []struct {
		name     string
		envVars  map[string]string
		expected map[string]FieldPopulation
	}{
		{
			name: "basic environment variable tracking",
			envVars: map[string]string{
				"APP_NAME":  "Test App",
				"APP_DEBUG": "true",
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
					SearchKeys:  []string{"APP_NAME"},
					FoundKey:    "APP_NAME",
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
					SearchKeys:  []string{"APP_DEBUG"},
					FoundKey:    "APP_DEBUG",
				},
			},
		},
	}

	for _, tt := range tests {
		// Set env for this test case
		for key, value := range tt.envVars {
            t.Setenv(key, value)
		}
		t.Run(tt.name, func(t *testing.T) {
            // Subtest does not call t.Setenv, but the parent did so we also avoid t.Parallel here to
            // keep semantics simple and consistent (can't parallelize parent anyway). If additional
            // cases without env mutation are added we can split them into a separate parallel test.

			// Create logger that captures debug output
			mockLogger := new(MockLogger)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

			// Create field tracker
			tracker := NewDefaultFieldTracker()
			tracker.SetLogger(mockLogger)

			// Create test configuration struct
			type TestConfig struct {
				AppName string `env:"APP_NAME"`
				Debug   bool   `env:"APP_DEBUG"`
			}

			config := &TestConfig{}

			// Create configuration builder with field tracking
			cfgBuilder := NewConfig()
			cfgBuilder.SetVerboseDebug(true, mockLogger)
			cfgBuilder.SetFieldTracker(tracker)

			// Add environment feeder - the AddFeeder method should set up field tracking automatically
			envFeeder := feeders.NewEnvFeeder()
			cfgBuilder.AddFeeder(envFeeder)

			// Add the configuration structure
			cfgBuilder.AddStructKey("test", config)

			// Feed configuration
			err := cfgBuilder.Feed()
			require.NoError(t, err)

			// Verify the config was populated correctly
			assert.Equal(t, "Test App", config.AppName)
			assert.True(t, config.Debug)

			// Field tracking should now work with the bridge
			// Verify field populations were tracked
			assert.GreaterOrEqual(t, len(tracker.FieldPopulations), 2, "Expected at least 2 field populations, got %d", len(tracker.FieldPopulations))

			// Check for specific field populations
			appNamePop := tracker.GetFieldPopulation("AppName")
			if assert.NotNil(t, appNamePop, "AppName field population should be tracked") {
				assert.Equal(t, "Test App", appNamePop.Value)
				assert.Equal(t, "env", appNamePop.SourceType)
				assert.Equal(t, "APP_NAME", appNamePop.SourceKey)
			}

			debugPop := tracker.GetFieldPopulation("Debug")
			if assert.NotNil(t, debugPop, "Debug field population should be tracked") {
				assert.Equal(t, true, debugPop.Value)
				assert.Equal(t, "env", debugPop.SourceType)
				assert.Equal(t, "APP_DEBUG", debugPop.SourceKey)
			}
		})
	}
}

// TestInstanceAwareFieldTracking tests instance-aware field tracking
func TestInstanceAwareFieldTracking(t *testing.T) {
	// Uses t.Setenv; cannot call t.Parallel on this *testing.T (Go 1.25 restriction).
	// Set up environment variables for instance-aware tracking
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":   "postgres",
		"DB_PRIMARY_DSN":      "postgres://localhost/primary",
		"DB_SECONDARY_DRIVER": "mysql",
		"DB_SECONDARY_DSN":    "mysql://localhost/secondary",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create logger that captures debug output
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create field tracker
	tracker := NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Create test configuration structures
	type ConnectionConfig struct {
		Driver string `env:"DRIVER"`
		DSN    string `env:"DSN"`
	}

	type DBConfig struct {
		Connections map[string]ConnectionConfig
	}

	dbConfig := &DBConfig{
		Connections: map[string]ConnectionConfig{
			"primary":   {},
			"secondary": {},
		},
	}

	// Create configuration builder with field tracking
	cfgBuilder := NewConfig()
	cfgBuilder.SetVerboseDebug(true, mockLogger)
	cfgBuilder.SetFieldTracker(tracker)

	// Add instance-aware environment feeder
	instanceAwareFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + strings.ToUpper(instanceKey) + "_"
	})
	cfgBuilder.AddFeeder(instanceAwareFeeder)

	// Add the configuration structure
	cfgBuilder.AddStructKey("db", dbConfig)

	// Feed configuration
	err := cfgBuilder.Feed()
	require.NoError(t, err)

	// Now use FeedInstances specifically for the connections map
	err = instanceAwareFeeder.FeedInstances(dbConfig.Connections)
	require.NoError(t, err)

	// Verify that config values were actually set
	assert.Equal(t, "postgres", dbConfig.Connections["primary"].Driver)
	assert.Equal(t, "postgres://localhost/primary", dbConfig.Connections["primary"].DSN)
	assert.Equal(t, "mysql", dbConfig.Connections["secondary"].Driver)
	assert.Equal(t, "mysql://localhost/secondary", dbConfig.Connections["secondary"].DSN)

	// Field tracking should now work - verify we have tracked populations
	assert.GreaterOrEqual(t, len(tracker.FieldPopulations), 4, "Should track at least 4 field populations (2 fields × 2 instances)")

	// Check for specific field populations
	var primaryDriverPop, primaryDSNPop, secondaryDriverPop, secondaryDSNPop *FieldPopulation
	for i := range tracker.FieldPopulations {
		pop := &tracker.FieldPopulations[i]
		if pop.FieldName == "Driver" && pop.InstanceKey == "primary" {
			primaryDriverPop = pop
		} else if pop.FieldName == "DSN" && pop.InstanceKey == "primary" {
			primaryDSNPop = pop
		} else if pop.FieldName == "Driver" && pop.InstanceKey == "secondary" {
			secondaryDriverPop = pop
		} else if pop.FieldName == "DSN" && pop.InstanceKey == "secondary" {
			secondaryDSNPop = pop
		}
	}

	// Verify primary instance tracking
	if assert.NotNil(t, primaryDriverPop, "Primary driver field population should be tracked") {
		assert.Equal(t, "postgres", primaryDriverPop.Value)
		assert.Equal(t, "env", primaryDriverPop.SourceType)
		assert.Equal(t, "DB_PRIMARY_DRIVER", primaryDriverPop.SourceKey)
		assert.Equal(t, "primary", primaryDriverPop.InstanceKey)
	}

	if assert.NotNil(t, primaryDSNPop, "Primary DSN field population should be tracked") {
		assert.Equal(t, "postgres://localhost/primary", primaryDSNPop.Value)
		assert.Equal(t, "env", primaryDSNPop.SourceType)
		assert.Equal(t, "DB_PRIMARY_DSN", primaryDSNPop.SourceKey)
		assert.Equal(t, "primary", primaryDSNPop.InstanceKey)
	}

	// Verify secondary instance tracking
	if assert.NotNil(t, secondaryDriverPop, "Secondary driver field population should be tracked") {
		assert.Equal(t, "mysql", secondaryDriverPop.Value)
		assert.Equal(t, "env", secondaryDriverPop.SourceType)
		assert.Equal(t, "DB_SECONDARY_DRIVER", secondaryDriverPop.SourceKey)
		assert.Equal(t, "secondary", secondaryDriverPop.InstanceKey)
	}

	if assert.NotNil(t, secondaryDSNPop, "Secondary DSN field population should be tracked") {
		assert.Equal(t, "mysql://localhost/secondary", secondaryDSNPop.Value)
		assert.Equal(t, "env", secondaryDSNPop.SourceType)
		assert.Equal(t, "DB_SECONDARY_DSN", secondaryDSNPop.SourceKey)
		assert.Equal(t, "secondary", secondaryDSNPop.InstanceKey)
	}
}
