package feeders

import (
	"os"
	"reflect"
	"testing"
)

// TestConsistentBehavior verifies that feeders behave consistently regardless of field tracking state
func TestConsistentBehavior(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		fileExt     string
	}{
		{
			name: "YAML_Feeder",
			fileContent: `
app:
  name: TestApp
  version: "1.0"
  debug: true
database:
  host: localhost
  port: 5432
`,
			fileExt: ".yaml",
		},
		{
			name: "JSON_Feeder",
			fileContent: `{
	"app": {
		"name": "TestApp",
		"version": "1.0",
		"debug": true
	},
	"database": {
		"host": "localhost",
		"port": 5432
	}
}`,
			fileExt: ".json",
		},
		{
			name: "TOML_Feeder",
			fileContent: `
[app]
name = "TestApp"
version = "1.0"
debug = true

[database]
host = "localhost"
port = 5432
`,
			fileExt: ".toml",
		},
	}

	type Config struct {
		App struct {
			Name    string `yaml:"name" json:"name" toml:"name"`
			Version string `yaml:"version" json:"version" toml:"version"`
			Debug   bool   `yaml:"debug" json:"debug" toml:"debug"`
		} `yaml:"app" json:"app" toml:"app"`
		Database struct {
			Host string `yaml:"host" json:"host" toml:"host"`
			Port int    `yaml:"port" json:"port" toml:"port"`
		} `yaml:"database" json:"database" toml:"database"`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tempFile, err := os.CreateTemp("", "test-*"+tt.fileExt)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.Write([]byte(tt.fileContent)); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tempFile.Close()

			// Test based on file extension
			switch tt.fileExt {
			case ".yaml":
				t.Run("YAML_consistency", func(t *testing.T) {
					testYAMLConsistency(t, tempFile.Name())
				})
			case ".json":
				t.Run("JSON_consistency", func(t *testing.T) {
					testJSONConsistency(t, tempFile.Name())
				})
			case ".toml":
				t.Run("TOML_consistency", func(t *testing.T) {
					testTOMLConsistency(t, tempFile.Name())
				})
			}
		})
	}
}

func testYAMLConsistency(t *testing.T, filePath string) {
	type Config struct {
		App struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
			Debug   bool   `yaml:"debug"`
		} `yaml:"app"`
		Database struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		} `yaml:"database"`
	}

	// Test without field tracking
	var configWithoutTracking Config
	feederWithoutTracking := NewYamlFeeder(filePath)
	err := feederWithoutTracking.Feed(&configWithoutTracking)
	if err != nil {
		t.Fatalf("Expected no error without field tracking, got %v", err)
	}

	// Test with field tracking
	var configWithTracking Config
	feederWithTracking := NewYamlFeeder(filePath)
	tracker := NewDefaultFieldTracker()
	feederWithTracking.SetFieldTracker(tracker)
	err = feederWithTracking.Feed(&configWithTracking)
	if err != nil {
		t.Fatalf("Expected no error with field tracking, got %v", err)
	}

	// Verify that both configs are identical
	if !reflect.DeepEqual(configWithoutTracking, configWithTracking) {
		t.Errorf("YAML configs should be identical regardless of field tracking state")
		t.Errorf("Without tracking: %+v", configWithoutTracking)
		t.Errorf("With tracking: %+v", configWithTracking)
	}

	verifyConfigValues(t, configWithTracking.App.Name, configWithTracking.App.Version, configWithTracking.App.Debug, configWithTracking.Database.Host, configWithTracking.Database.Port)

	// Verify field tracking recorded something (when enabled)
	populations := tracker.GetFieldPopulations()
	if len(populations) == 0 {
		t.Error("Expected field populations to be recorded when field tracking is enabled")
	}
}

func testJSONConsistency(t *testing.T, filePath string) {
	type Config struct {
		App struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Debug   bool   `json:"debug"`
		} `json:"app"`
		Database struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		} `json:"database"`
	}

	// Test without field tracking
	var configWithoutTracking Config
	feederWithoutTracking := NewJSONFeeder(filePath)
	err := feederWithoutTracking.Feed(&configWithoutTracking)
	if err != nil {
		t.Fatalf("Expected no error without field tracking, got %v", err)
	}

	// Test with field tracking
	var configWithTracking Config
	feederWithTracking := NewJSONFeeder(filePath)
	tracker := NewDefaultFieldTracker()
	feederWithTracking.SetFieldTracker(tracker)
	err = feederWithTracking.Feed(&configWithTracking)
	if err != nil {
		t.Fatalf("Expected no error with field tracking, got %v", err)
	}

	// Verify that both configs are identical
	if !reflect.DeepEqual(configWithoutTracking, configWithTracking) {
		t.Errorf("JSON configs should be identical regardless of field tracking state")
		t.Errorf("Without tracking: %+v", configWithoutTracking)
		t.Errorf("With tracking: %+v", configWithTracking)
	}

	verifyConfigValues(t, configWithTracking.App.Name, configWithTracking.App.Version, configWithTracking.App.Debug, configWithTracking.Database.Host, configWithTracking.Database.Port)

	// Verify field tracking recorded something (when enabled)
	populations := tracker.GetFieldPopulations()
	if len(populations) == 0 {
		t.Error("Expected field populations to be recorded when field tracking is enabled")
	}
}

func testTOMLConsistency(t *testing.T, filePath string) {
	type Config struct {
		App struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
			Debug   bool   `toml:"debug"`
		} `toml:"app"`
		Database struct {
			Host string `toml:"host"`
			Port int    `toml:"port"`
		} `toml:"database"`
	}

	// Test without field tracking
	var configWithoutTracking Config
	feederWithoutTracking := NewTomlFeeder(filePath)
	err := feederWithoutTracking.Feed(&configWithoutTracking)
	if err != nil {
		t.Fatalf("Expected no error without field tracking, got %v", err)
	}

	// Test with field tracking
	var configWithTracking Config
	feederWithTracking := NewTomlFeeder(filePath)
	tracker := NewDefaultFieldTracker()
	feederWithTracking.SetFieldTracker(tracker)
	err = feederWithTracking.Feed(&configWithTracking)
	if err != nil {
		t.Fatalf("Expected no error with field tracking, got %v", err)
	}

	// Verify that both configs are identical
	if !reflect.DeepEqual(configWithoutTracking, configWithTracking) {
		t.Errorf("TOML configs should be identical regardless of field tracking state")
		t.Errorf("Without tracking: %+v", configWithoutTracking)
		t.Errorf("With tracking: %+v", configWithTracking)
	}

	verifyConfigValues(t, configWithTracking.App.Name, configWithTracking.App.Version, configWithTracking.App.Debug, configWithTracking.Database.Host, configWithTracking.Database.Port)

	// Verify field tracking recorded something (when enabled)
	populations := tracker.GetFieldPopulations()
	if len(populations) == 0 {
		t.Error("Expected field populations to be recorded when field tracking is enabled")
	}
}

func verifyConfigValues(t *testing.T, name, version string, debug bool, host string, port int) {
	if name != "TestApp" {
		t.Errorf("Expected App.Name to be 'TestApp', got '%s'", name)
	}
	if version != "1.0" {
		t.Errorf("Expected App.Version to be '1.0', got '%s'", version)
	}
	if !debug {
		t.Errorf("Expected App.Debug to be true, got false")
	}
	if host != "localhost" {
		t.Errorf("Expected Database.Host to be 'localhost', got '%s'", host)
	}
	if port != 5432 {
		t.Errorf("Expected Database.Port to be 5432, got %d", port)
	}
}
