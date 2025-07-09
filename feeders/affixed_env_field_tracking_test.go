package feeders

import (
	"fmt"
	"os"
	"testing"
)

// TestConfig struct for AffixedEnv field tracking tests
type TestAffixedEnvConfig struct {
	Name    string `env:"NAME"`
	Port    int    `env:"PORT"`
	Enabled bool   `env:"ENABLED"`
	Debug   string `env:"DEBUG"`
}

func TestAffixedEnvFeeder_FieldTracking(t *testing.T) {
	// Set up environment variables with prefix and suffix
	envVars := map[string]string{
		"APP__NAME__PROD":    "test-app",
		"APP__PORT__PROD":    "8080",
		"APP__ENABLED__PROD": "true",
		"APP__DEBUG__PROD":   "verbose",
		"OTHER_VAR":          "ignored", // Should not be matched
	}

	// Set environment variables for test
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// Clean up after test
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Create feeder and field tracker
	feeder := NewAffixedEnvFeeder("APP_", "_PROD")
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	// Test config structure
	var config TestAffixedEnvConfig

	// Feed the configuration
	err := feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed config: %v", err)
	}

	// Verify configuration was populated correctly
	if config.Name != "test-app" {
		t.Errorf("Expected Name to be 'test-app', got %s", config.Name)
	}
	if config.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", config.Port)
	}
	if !config.Enabled {
		t.Errorf("Expected Enabled to be true, got %v", config.Enabled)
	}
	if config.Debug != "verbose" {
		t.Errorf("Expected Debug to be 'verbose', got %s", config.Debug)
	}

	// Get field populations
	populations := tracker.GetFieldPopulations()

	// Verify we have tracking information for all fields
	expectedFields := []string{"Name", "Port", "Enabled", "Debug"}

	for _, fieldPath := range expectedFields {
		found := false
		for _, pop := range populations {
			if pop.FieldPath == fieldPath {
				found = true
				// Verify basic tracking information
				if pop.FeederType != "AffixedEnvFeeder" {
					t.Errorf("Expected FeederType 'AffixedEnvFeeder' for field %s, got %s", fieldPath, pop.FeederType)
				}
				if pop.SourceType != "env_affixed" {
					t.Errorf("Expected SourceType 'env_affixed' for field %s, got %s", fieldPath, pop.SourceType)
				}
				if pop.SourceKey == "" {
					t.Errorf("Expected non-empty SourceKey for field %s", fieldPath)
				}
				if pop.Value == nil {
					t.Errorf("Expected non-nil Value for field %s", fieldPath)
				}
				break
			}
		}
		if !found {
			t.Errorf("Field tracking not found for field: %s", fieldPath)
		}
	}

	// Verify specific field values and source keys in tracking
	for _, pop := range populations {
		switch pop.FieldPath {
		case "Name":
			if fmt.Sprintf("%v", pop.Value) != "test-app" {
				t.Errorf("Expected tracked value 'test-app' for Name, got %v", pop.Value)
			}
			if pop.SourceKey != "APP__NAME__PROD" {
				t.Errorf("Expected SourceKey 'APP__NAME__PROD' for Name, got %s", pop.SourceKey)
			}
		case "Port":
			if fmt.Sprintf("%v", pop.Value) != "8080" {
				t.Errorf("Expected tracked value '8080' for Port, got %v", pop.Value)
			}
			if pop.SourceKey != "APP__PORT__PROD" {
				t.Errorf("Expected SourceKey 'APP__PORT__PROD' for Port, got %s", pop.SourceKey)
			}
		case "Enabled":
			if fmt.Sprintf("%v", pop.Value) != "true" {
				t.Errorf("Expected tracked value 'true' for Enabled, got %v", pop.Value)
			}
			if pop.SourceKey != "APP__ENABLED__PROD" {
				t.Errorf("Expected SourceKey 'APP__ENABLED__PROD' for Enabled, got %s", pop.SourceKey)
			}
		case "Debug":
			if fmt.Sprintf("%v", pop.Value) != "verbose" {
				t.Errorf("Expected tracked value 'verbose' for Debug, got %v", pop.Value)
			}
			if pop.SourceKey != "APP__DEBUG__PROD" {
				t.Errorf("Expected SourceKey 'APP__DEBUG__PROD' for Debug, got %s", pop.SourceKey)
			}
		}
	}
}

func TestAffixedEnvFeeder_SetFieldTracker(t *testing.T) {
	feeder := NewAffixedEnvFeeder("PREFIX_", "_SUFFIX")
	tracker := NewDefaultFieldTracker()

	// Test that SetFieldTracker method exists and can be called
	feeder.SetFieldTracker(tracker)

	// The actual tracking functionality is tested in TestAffixedEnvFeeder_FieldTracking
}

func TestAffixedEnvFeeder_WithoutFieldTracker(t *testing.T) {
	// Set up environment variables
	os.Setenv("TEST__NAME__DEV", "test-app")
	os.Setenv("TEST__PORT__DEV", "8080")
	defer func() {
		os.Unsetenv("TEST__NAME__DEV")
		os.Unsetenv("TEST__PORT__DEV")
	}()

	// Create feeder without field tracker
	feeder := NewAffixedEnvFeeder("TEST_", "_DEV")

	var config TestAffixedEnvConfig

	// Should work without field tracker
	err := feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed config without field tracker: %v", err)
	}

	// Verify configuration was populated correctly
	if config.Name != "test-app" {
		t.Errorf("Expected Name to be 'test-app', got %s", config.Name)
	}
	if config.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", config.Port)
	}
}
