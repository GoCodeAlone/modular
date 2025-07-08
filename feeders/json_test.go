package feeders

import (
	"os"
	"testing"
)

func TestJSONFeeder_Feed(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	jsonContent := `{
		"App": {
			"Name": "TestApp",
			"Version": "1.0",
			"Debug": true
		}
	}`
	if _, err := tempFile.Write([]byte(jsonContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name    string `json:"Name"`
			Version string `json:"Version"`
			Debug   bool   `json:"Debug"`
		}
	}

	var config Config
	feeder := NewJSONFeeder(tempFile.Name())
	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if config.App.Name != "TestApp" {
		t.Errorf("Expected Name to be 'TestApp', got '%s'", config.App.Name)
	}
	if config.App.Version != "1.0" {
		t.Errorf("Expected Version to be '1.0', got '%s'", config.App.Version)
	}
	if !config.App.Debug {
		t.Errorf("Expected Debug to be true, got false")
	}
}

func TestJSONFeeder_FeedKey(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	jsonContent := `{
		"App": {
			"Name": "TestApp",
			"Version": "1.0"
		}
	}`
	if _, err := tempFile.Write([]byte(jsonContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type AppConfig struct {
		Name    string `json:"Name"`
		Version string `json:"Version"`
	}
	var appConfig AppConfig
	feeder := NewJSONFeeder(tempFile.Name())
	err = feeder.FeedKey("App", &appConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if appConfig.Name != "TestApp" {
		t.Errorf("Expected Name to be 'TestApp', got '%s'", appConfig.Name)
	}
	if appConfig.Version != "1.0" {
		t.Errorf("Expected Version to be '1.0', got '%s'", appConfig.Version)
	}
}
