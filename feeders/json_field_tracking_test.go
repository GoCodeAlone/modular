package feeders

import (
	"fmt"
	"os"
	"testing"
)

// TestConfig struct for JSON field tracking tests
type TestJSONConfig struct {
	Name    string       `json:"name"`
	Port    int          `json:"port"`
	Enabled bool         `json:"enabled"`
	Tags    []string     `json:"tags"`
	DB      TestDBConfig `json:"db"`
}

type TestDBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
}

func TestJSONFeeder_FieldTracking(t *testing.T) {
	// Create test JSON file
	jsonContent := `{
	"name": "test-app",
	"port": 8080,
	"enabled": true,
	"tags": ["web", "api"],
	"db": {
		"host": "localhost",
		"port": 5432,
		"database": "testdb"
	}
}`

	// Create temporary JSON file
	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create feeder and field tracker
	feeder := NewJSONFeeder(tmpFile.Name())
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	// Test config structure
	var config TestJSONConfig

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
	if len(config.Tags) != 2 || config.Tags[0] != "web" || config.Tags[1] != "api" {
		t.Errorf("Expected Tags to be ['web', 'api'], got %v", config.Tags)
	}
	if config.DB.Host != "localhost" {
		t.Errorf("Expected DB.Host to be 'localhost', got %s", config.DB.Host)
	}
	if config.DB.Port != 5432 {
		t.Errorf("Expected DB.Port to be 5432, got %d", config.DB.Port)
	}
	if config.DB.Database != "testdb" {
		t.Errorf("Expected DB.Database to be 'testdb', got %s", config.DB.Database)
	}

	// Get field populations
	populations := tracker.GetFieldPopulations()

	// Verify we have tracking information for all fields
	expectedFields := []string{"Name", "Port", "Enabled", "Tags", "DB.Host", "DB.Port", "DB.Database"}

	for _, fieldPath := range expectedFields {
		found := false
		for _, pop := range populations {
			if pop.FieldPath == fieldPath {
				found = true
				// Verify basic tracking information
				if pop.FeederType != "JSONFeeder" {
					t.Errorf("Expected FeederType 'JSONFeeder' for field %s, got %s", fieldPath, pop.FeederType)
				}
				if pop.SourceType != "json_file" {
					t.Errorf("Expected SourceType 'json_file' for field %s, got %s", fieldPath, pop.SourceType)
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
		case "DB.Host":
			if fmt.Sprintf("%v", pop.Value) != "localhost" {
				t.Errorf("Expected tracked value 'localhost' for DB.Host, got %v", pop.Value)
			}
		}
	}
}

func TestJSONFeeder_SetFieldTracker(t *testing.T) {
	feeder := NewJSONFeeder("test.json")
	tracker := NewDefaultFieldTracker()

	// Test that SetFieldTracker method exists and can be called
	feeder.SetFieldTracker(tracker)

	// The actual tracking functionality is tested in TestJSONFeeder_FieldTracking
}

func TestJSONFeeder_WithoutFieldTracker(t *testing.T) {
	// Create test JSON file
	jsonContent := `{"name": "test-app", "port": 8080}`

	tmpFile, err := os.CreateTemp("", "test_config_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Create feeder without field tracker
	feeder := NewJSONFeeder(tmpFile.Name())

	var config TestJSONConfig

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
