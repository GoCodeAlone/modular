package feeders

import (
	"testing"
)

type MockLogger struct {
	messages []string
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.messages = append(m.messages, msg)
}

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

	t.Run("verbose debugging", func(t *testing.T) {
		t.Setenv("TEST_VALUE", "verbose_test")

		type Config struct {
			TestValue string `env:"TEST_VALUE"`
		}

		var config Config
		feeder := NewEnvFeeder()
		logger := &MockLogger{}

		// Enable verbose debugging
		feeder.SetVerboseDebug(true, logger)

		err := feeder.Feed(&config)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if config.TestValue != "verbose_test" {
			t.Errorf("Expected TestValue to be 'verbose_test', got '%s'", config.TestValue)
		}

		// Check that verbose debug messages were logged
		if len(logger.messages) == 0 {
			t.Error("Expected verbose debug messages to be logged")
		}

		// Check for specific expected messages
		expectedMessages := []string{
			"Verbose environment feeder debugging enabled",
			"EnvFeeder: Starting feed process",
			"EnvFeeder: Processing struct",
			"EnvFeeder: Feed completed successfully",
		}

		for _, expected := range expectedMessages {
			found := false
			for _, msg := range logger.messages {
				if msg == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected debug message '%s' not found in logged messages", expected)
			}
		}
	})

	t.Run("verbose debugging disabled", func(t *testing.T) {
		t.Setenv("TEST_VALUE", "no_verbose_test")

		type Config struct {
			TestValue string `env:"TEST_VALUE"`
		}

		var config Config
		feeder := NewEnvFeeder()
		logger := &MockLogger{}

		// Verbose debugging is disabled by default
		err := feeder.Feed(&config)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if config.TestValue != "no_verbose_test" {
			t.Errorf("Expected TestValue to be 'no_verbose_test', got '%s'", config.TestValue)
		}

		// Check that no verbose debug messages were logged
		if len(logger.messages) > 0 {
			t.Error("Expected no debug messages when verbose debugging is disabled")
		}
	})
}
