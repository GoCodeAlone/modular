package feeders

import (
	"testing"
)

func TestEnvFeeder(t *testing.T) {
	t.Run("read environment variables", func(t *testing.T) {
		t.Setenv("APP_NAME", "TestApp")
		t.Setenv("APP_VERSION", "1.0")
		t.Setenv("APP_DEBUG", "true")

		type Config struct {
			App struct {
				Name    string `env:"APP_NAME"`
				Version string `env:"APP_VERSION"`
				Debug   bool   `env:"APP_DEBUG"`
			}
		}

		var config Config
		feeder := NewEnvFeeder()
		err := feeder.Feed(&config)

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
	})

	t.Run("missing environment variables", func(t *testing.T) {
		type Config struct {
			MissingVar string
		}

		var config Config
		feeder := NewEnvFeeder()
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error for missing env var, got %v", err)
		}
		if config.MissingVar != "" {
			t.Errorf("Expected MissingVar to be empty, got '%s'", config.MissingVar)
		}
	})
}
