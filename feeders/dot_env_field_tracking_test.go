package feeders

import (
	"fmt"
	"os"
	"testing"
)

// TestConfig struct for DotEnv field tracking tests
type TestDotEnvConfig struct {
	Name    string `env:"NAME"`
	Port    int    `env:"PORT"`
	Enabled bool   `env:"ENABLED"`
	Debug   string `env:"DEBUG"`
}

func TestDotEnvFeeder_FieldTracking(t *testing.T) {
	// Create test .env file
	envContent := `NAME=test-app
PORT=8080
ENABLED=true
DEBUG=verbose
UNUSED_VAR=ignored
`

	// Create temporary .env file
	tmpFile, err := os.CreateTemp("", "test_config_*.env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(envContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create feeder and field tracker
	feeder := NewDotEnvFeeder(tmpFile.Name())
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	// Test config structure
	var config TestDotEnvConfig

	// Feed the configuration
	err = feeder.Feed(&config)
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
				if pop.FeederType != "DotEnvFeeder" {
					t.Errorf("Expected FeederType 'DotEnvFeeder' for field %s, got %s", fieldPath, pop.FeederType)
				}
				if pop.SourceType != "dot_env_file" {
					t.Errorf("Expected SourceType 'dot_env_file' for field %s, got %s", fieldPath, pop.SourceType)
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

	// Verify specific field values in tracking
	for _, pop := range populations {
		switch pop.FieldPath {
		case "Name":
			if fmt.Sprintf("%v", pop.Value) != "test-app" {
				t.Errorf("Expected tracked value 'test-app' for Name, got %v", pop.Value)
			}
		case "Port":
			if fmt.Sprintf("%v", pop.Value) != "8080" {
				t.Errorf("Expected tracked value '8080' for Port, got %v", pop.Value)
			}
		case "Enabled":
			if fmt.Sprintf("%v", pop.Value) != "true" {
				t.Errorf("Expected tracked value 'true' for Enabled, got %v", pop.Value)
			}
		case "Debug":
			if fmt.Sprintf("%v", pop.Value) != "verbose" {
				t.Errorf("Expected tracked value 'verbose' for Debug, got %v", pop.Value)
			}
		}
	}
}

func TestDotEnvFeeder_SetFieldTracker(t *testing.T) {
	feeder := NewDotEnvFeeder("test.env")
	tracker := NewDefaultFieldTracker()

	// Test that SetFieldTracker method exists and can be called
	feeder.SetFieldTracker(tracker)

	// The actual tracking functionality is tested in TestDotEnvFeeder_FieldTracking
}

func TestDotEnvFeeder_WithoutFieldTracker(t *testing.T) {
	// Create test .env file
	envContent := `NAME=test-app
PORT=8080`

	tmpFile, err := os.CreateTemp("", "test_config_*.env")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(envContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create feeder without field tracker
	feeder := NewDotEnvFeeder(tmpFile.Name())

	var config TestDotEnvConfig

	// Should work without field tracker
	err = feeder.Feed(&config)
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
