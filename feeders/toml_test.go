package feeders

import (
	"os"
	"testing"
)

func TestTomlFeeder_Feed(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	tomlContent := `
[App]
Name = "TestApp"
Version = "1.0"
Debug = true
`
	if _, err := tempFile.Write([]byte(tomlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	type Config struct {
		App struct {
			Name    string `toml:"Name"`
			Version string `toml:"Version"`
			Debug   bool   `toml:"Debug"`
		}
	}

	var config Config
	feeder := NewTomlFeeder(tempFile.Name())
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
