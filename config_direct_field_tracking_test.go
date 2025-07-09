package modular

import (
	"strings"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestDirectFeederFieldTracking tests field tracking when calling feeder.Feed() directly
func TestDirectFeederFieldTracking(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "basic environment variable tracking",
			envVars: map[string]string{
				"APP_NAME":  "Test App",
				"APP_DEBUG": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Create logger that captures debug output
			mockLogger := new(MockLogger)
			mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

			// Create field tracker
			tracker := NewDefaultFieldTracker()
			tracker.SetLogger(mockLogger)

			// Create test configuration struct
			type TestConfig struct {
				App struct {
					Name  string `env:"APP_NAME"`
					Debug bool   `env:"APP_DEBUG"`
				}
			}

			config := &TestConfig{}

			// Create environment feeder with field tracking
			envFeeder := feeders.NewEnvFeeder()
			envFeeder.SetVerboseDebug(true, mockLogger)

			// Set up field tracking bridge
			bridge := NewFieldTrackingBridge(tracker)
			envFeeder.SetFieldTracker(bridge)

			// Feed configuration directly
			err := envFeeder.Feed(config)
			require.NoError(t, err)

			// Verify that config values were actually set
			assert.Equal(t, "Test App", config.App.Name)
			assert.True(t, config.App.Debug)

			// Verify that field populations were tracked
			assert.NotEmpty(t, tracker.FieldPopulations, "Should have tracked field populations")

			// Print tracked populations for debugging
			t.Logf("Tracked %d field populations:", len(tracker.FieldPopulations))
			for i, fp := range tracker.FieldPopulations {
				t.Logf("  %d: %s -> %v (from %s:%s)", i, fp.FieldPath, fp.Value, fp.SourceType, fp.SourceKey)
			}
		})
	}
}

// TestInstanceAwareDirectFieldTracking tests instance-aware field tracking with direct feeding
func TestInstanceAwareDirectFieldTracking(t *testing.T) {
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

	// Test the primary connection first
	primaryConfig := &ConnectionConfig{}

	// Create instance-aware environment feeder
	instanceAwareFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + strings.ToUpper(instanceKey) + "_"
	})
	instanceAwareFeeder.SetVerboseDebug(true, mockLogger)

	// Set up field tracking bridge
	bridge := NewFieldTrackingBridge(tracker)
	instanceAwareFeeder.SetFieldTracker(bridge)

	// Feed primary configuration
	err := instanceAwareFeeder.FeedKey("primary", primaryConfig)
	require.NoError(t, err)

	// Verify that config values were actually set
	assert.Equal(t, "postgres", primaryConfig.Driver)
	assert.Equal(t, "postgres://localhost/primary", primaryConfig.DSN)

	// Test secondary connection
	secondaryConfig := &ConnectionConfig{}
	err = instanceAwareFeeder.FeedKey("secondary", secondaryConfig)
	require.NoError(t, err)

	// Verify that config values were actually set
	assert.Equal(t, "mysql", secondaryConfig.Driver)
	assert.Equal(t, "mysql://localhost/secondary", secondaryConfig.DSN)

	// Verify that field populations were tracked
	assert.NotEmpty(t, tracker.FieldPopulations, "Should have tracked field populations")

	// Print tracked populations for debugging
	t.Logf("Tracked %d field populations:", len(tracker.FieldPopulations))
	for i, fp := range tracker.FieldPopulations {
		t.Logf("  %d: %s -> %v (from %s:%s, instance:%s)", i, fp.FieldPath, fp.Value, fp.SourceType, fp.SourceKey, fp.InstanceKey)
	}

	// Verify specific field populations
	primaryDriverPop := tracker.GetFieldPopulation("Driver")
	if primaryDriverPop != nil {
		assert.Equal(t, "Driver", primaryDriverPop.FieldName)
		assert.Equal(t, "env", primaryDriverPop.SourceType)
		assert.Equal(t, "DB_PRIMARY_DRIVER", primaryDriverPop.SourceKey)
		assert.Equal(t, "postgres", primaryDriverPop.Value)
		assert.Equal(t, "primary", primaryDriverPop.InstanceKey)
	}
}
