package feeders

import (
	"strings"
	"testing"
)

// Mock logger for testing
type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(msg string, args ...any) {
	m.logs = append(m.logs, msg)
}

func TestVerboseEnvFeeder(t *testing.T) {
	t.Run("read environment variables with verbose logging", func(t *testing.T) {
		t.Setenv("APP_NAME", "TestApp")
		t.Setenv("APP_VERSION", "1.0")
		t.Setenv("APP_DEBUG", "true")

		logger := &mockLogger{}

		type Config struct {
			App struct {
				Name    string `env:"APP_NAME"`
				Version string `env:"APP_VERSION"`
				Debug   bool   `env:"APP_DEBUG"`
			}
		}

		var config Config
		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)
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

		// Check that verbose logging was enabled
		if len(logger.logs) == 0 {
			t.Error("Expected verbose logs to be generated")
		}

		// Check that debug messages were logged
		foundStartMsg := false
		foundCompleteMsg := false
		for _, log := range logger.logs {
			if strings.Contains(log, "Starting feed process") {
				foundStartMsg = true
			}
			if strings.Contains(log, "Feed completed successfully") {
				foundCompleteMsg = true
			}
		}

		if !foundStartMsg {
			t.Error("Expected to find 'Starting feed process' log message")
		}
		if !foundCompleteMsg {
			t.Error("Expected to find 'Feed completed successfully' log message")
		}
	})

	t.Run("verbose logging disabled", func(t *testing.T) {
		t.Setenv("TEST_VAR", "test_value")

		logger := &mockLogger{}

		type Config struct {
			TestVar string `env:"TEST_VAR"`
		}

		var config Config
		feeder := NewVerboseEnvFeeder()
		// Don't enable verbose logging
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.TestVar != "test_value" {
			t.Errorf("Expected TestVar to be 'test_value', got '%s'", config.TestVar)
		}

		// Check that no logs were generated
		if len(logger.logs) > 0 {
			t.Errorf("Expected no logs when verbose logging is disabled, got %d logs", len(logger.logs))
		}
	})

	t.Run("invalid structure", func(t *testing.T) {
		logger := &mockLogger{}

		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)

		// Test with non-pointer
		var config struct {
			Name string `env:"NAME"`
		}
		err := feeder.Feed(config)
		if err == nil {
			t.Error("Expected error for non-pointer structure")
		}

		// Test with nil
		err = feeder.Feed(nil)
		if err == nil {
			t.Error("Expected error for nil structure")
		}

		// Test with pointer to non-struct
		var name string
		err = feeder.Feed(&name)
		if err == nil {
			t.Error("Expected error for pointer to non-struct")
		}
	})

	t.Run("nested struct processing", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_PORT", "5432")

		logger := &mockLogger{}

		type Database struct {
			Host string `env:"DB_HOST"`
			Port int    `env:"DB_PORT"`
		}

		type Config struct {
			DB Database
		}

		var config Config
		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.DB.Host != "localhost" {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.DB.Host)
		}
		if config.DB.Port != 5432 {
			t.Errorf("Expected Port to be 5432, got %d", config.DB.Port)
		}

		// Check that nested struct processing was logged
		foundNestedMsg := false
		for _, log := range logger.logs {
			if strings.Contains(log, "Processing nested struct") {
				foundNestedMsg = true
				break
			}
		}
		if !foundNestedMsg {
			t.Error("Expected to find 'Processing nested struct' log message")
		}
	})

	t.Run("pointer to struct processing", func(t *testing.T) {
		t.Setenv("API_KEY", "secret123")

		logger := &mockLogger{}

		type Auth struct {
			APIKey string `env:"API_KEY"`
		}

		type Config struct {
			Auth *Auth
		}

		var config Config
		config.Auth = &Auth{} // Initialize the pointer

		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.Auth.APIKey != "secret123" {
			t.Errorf("Expected APIKey to be 'secret123', got '%s'", config.Auth.APIKey)
		}
	})

	t.Run("missing environment variables", func(t *testing.T) {
		logger := &mockLogger{}

		type Config struct {
			MissingVar string `env:"MISSING_VAR"`
		}

		var config Config
		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error for missing env var, got %v", err)
		}
		if config.MissingVar != "" {
			t.Errorf("Expected MissingVar to be empty, got '%s'", config.MissingVar)
		}

		// Check that missing variable was logged
		foundMissingMsg := false
		for _, log := range logger.logs {
			if strings.Contains(log, "Environment variable not found or empty") {
				foundMissingMsg = true
				break
			}
		}
		if !foundMissingMsg {
			t.Error("Expected to find 'Environment variable not found or empty' log message")
		}
	})

	t.Run("field without env tag", func(t *testing.T) {
		logger := &mockLogger{}

		type Config struct {
			FieldWithoutTag string
		}

		var config Config
		feeder := NewVerboseEnvFeeder()
		feeder.SetVerboseDebug(true, logger)
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check that no env tag was logged
		foundNoTagMsg := false
		for _, log := range logger.logs {
			if strings.Contains(log, "No env tag found") {
				foundNoTagMsg = true
				break
			}
		}
		if !foundNoTagMsg {
			t.Error("Expected to find 'No env tag found' log message")
		}
	})
}
