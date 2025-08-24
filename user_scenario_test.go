package modular_test

import (
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLogger for capturing debug output
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.Called(msg, args)
}

// TestUserScenario tests the exact scenario the user described:
// Database module DSN configuration not being populated from ENV vars
// in instance-aware scenarios, with verbose logging to show exactly
// what's being looked for.
func TestUserScenario_DatabaseDSNInstanceAware(t *testing.T) {
	// This test demonstrates that the user can now get complete visibility
	// into field-level configuration population, especially for Database
	// module DSN values with instance-aware environment variables.

	// Set up the exact environment variables the user would have
	envVars := map[string]string{
		"DB_PRIMARY_DRIVER":      "postgres",
		"DB_PRIMARY_DSN":         "postgres://user:pass@localhost/primary_db",
		"DB_PRIMARY_MAX_CONNS":   "25",
		"DB_SECONDARY_DRIVER":    "mysql",
		"DB_SECONDARY_DSN":       "mysql://user:pass@localhost/secondary_db",
		"DB_SECONDARY_MAX_CONNS": "15",
		"DB_CACHE_DRIVER":        "redis",
		"DB_CACHE_DSN":           "redis://localhost:6379/0",
		"DB_CACHE_MAX_CONNS":     "10",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	// Create a logger that captures all debug output
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()

	// Create field tracker to get detailed field population information
	tracker := modular.NewDefaultFieldTracker()
	tracker.SetLogger(mockLogger)

	// Define the Database module configuration structure
	// (matching the actual Database module)
	type ConnectionConfig struct {
		Driver   string `env:"DRIVER"`
		DSN      string `env:"DSN"` // This is the field the user was having trouble with
		MaxConns int    `env:"MAX_CONNS"`
	}

	type DatabaseConfig struct {
		Connections map[string]ConnectionConfig `yaml:"connections"`
		Default     string                      `yaml:"default"`
	}

	// Initialize configuration with multiple database instances
	dbConfig := &DatabaseConfig{
		Connections: map[string]ConnectionConfig{
			"primary":   {},
			"secondary": {},
			"cache":     {},
		},
		Default: "primary",
	}

	// Create configuration builder with verbose debugging and field tracking
	cfg := modular.NewConfig()
	cfg.SetVerboseDebug(true, mockLogger)
	cfg.SetFieldTracker(tracker)

	// Add instance-aware environment feeder - this is what enables
	// instance-specific environment variable mapping
	instanceFeeder := feeders.NewInstanceAwareEnvFeeder(func(instanceKey string) string {
		return "DB_" + strings.ToUpper(instanceKey) + "_"
	})
	cfg.AddFeeder(instanceFeeder)

	// Add the database configuration
	cfg.AddStructKey("database", dbConfig)

	// Feed configuration - this will populate from environment variables
	err := cfg.Feed()
	require.NoError(t, err)

	// Use FeedInstances to populate the connections map with instance-aware values
	err = instanceFeeder.FeedInstances(dbConfig.Connections)
	require.NoError(t, err)

	// === VERIFICATION: Configuration was populated correctly ===

	// Primary database connection
	primaryConn := dbConfig.Connections["primary"]
	assert.Equal(t, "postgres", primaryConn.Driver)
	assert.Equal(t, "postgres://user:pass@localhost/primary_db", primaryConn.DSN)
	assert.Equal(t, 25, primaryConn.MaxConns)

	// Secondary database connection
	secondaryConn := dbConfig.Connections["secondary"]
	assert.Equal(t, "mysql", secondaryConn.Driver)
	assert.Equal(t, "mysql://user:pass@localhost/secondary_db", secondaryConn.DSN)
	assert.Equal(t, 15, secondaryConn.MaxConns)

	// Cache database connection
	cacheConn := dbConfig.Connections["cache"]
	assert.Equal(t, "redis", cacheConn.Driver)
	assert.Equal(t, "redis://localhost:6379/0", cacheConn.DSN)
	assert.Equal(t, 10, cacheConn.MaxConns)

	// === VERIFICATION: Field tracking provides complete visibility ===

	populations := tracker.FieldPopulations
	require.GreaterOrEqual(t, len(populations), 9, "Should track 3 fields × 3 instances = 9 populations")

	t.Logf("=== FIELD TRACKING RESULTS ===")
	t.Logf("Tracked %d field populations:", len(populations))

	for i, pop := range populations {
		t.Logf("  %d: %s.%s -> %v", i+1, pop.InstanceKey, pop.FieldName, pop.Value)
		t.Logf("     Source: %s:%s", pop.SourceType, pop.SourceKey)
		t.Logf("     Searched: %v", pop.SearchKeys)
		t.Logf("     Found: %s", pop.FoundKey)
		t.Logf("")
	}

	// Specifically verify the user's problem case: DSN field tracking
	primaryDSNPop := findInstanceFieldPopulation(populations, "DSN", "primary")
	require.NotNil(t, primaryDSNPop, "Primary DSN field population should be tracked")
	assert.Equal(t, "postgres://user:pass@localhost/primary_db", primaryDSNPop.Value)
	assert.Equal(t, "env", primaryDSNPop.SourceType)
	assert.Equal(t, "DB_PRIMARY_DSN", primaryDSNPop.SourceKey)
	assert.Equal(t, "primary", primaryDSNPop.InstanceKey)
	assert.Contains(t, primaryDSNPop.SearchKeys, "DB_PRIMARY_DSN")
	assert.Equal(t, "DB_PRIMARY_DSN", primaryDSNPop.FoundKey)

	secondaryDSNPop := findInstanceFieldPopulation(populations, "DSN", "secondary")
	require.NotNil(t, secondaryDSNPop, "Secondary DSN field population should be tracked")
	assert.Equal(t, "mysql://user:pass@localhost/secondary_db", secondaryDSNPop.Value)
	assert.Equal(t, "env", secondaryDSNPop.SourceType)
	assert.Equal(t, "DB_SECONDARY_DSN", secondaryDSNPop.SourceKey)
	assert.Equal(t, "secondary", secondaryDSNPop.InstanceKey)

	cacheDSNPop := findInstanceFieldPopulation(populations, "DSN", "cache")
	require.NotNil(t, cacheDSNPop, "Cache DSN field population should be tracked")
	assert.Equal(t, "redis://localhost:6379/0", cacheDSNPop.Value)
	assert.Equal(t, "env", cacheDSNPop.SourceType)
	assert.Equal(t, "DB_CACHE_DSN", cacheDSNPop.SourceKey)
	assert.Equal(t, "cache", cacheDSNPop.InstanceKey)

	// === VERIFICATION: Verbose debug logging captured everything ===

	// Verify that verbose debug logging was used
	mockLogger.AssertCalled(t, "Debug", mock.MatchedBy(func(msg string) bool {
		return msg == "Field populated"
	}), mock.Anything)

	t.Logf("=== SUCCESS ===")
	t.Logf("✅ User's issue resolved:")
	t.Logf("   - Database DSN fields populated from instance-aware ENV vars")
	t.Logf("   - Complete field-level tracking shows exactly which ENV vars were used")
	t.Logf("   - Verbose logging provides step-by-step visibility into the process")
	t.Logf("   - Each field shows: source type, source key, search keys, found key, instance")
}

// Helper function to find a field population by field name and instance key
func findInstanceFieldPopulation(populations []modular.FieldPopulation, fieldName, instanceKey string) *modular.FieldPopulation {
	for _, pop := range populations {
		if pop.FieldName == fieldName && pop.InstanceKey == instanceKey {
			return &pop
		}
	}
	return nil
}
