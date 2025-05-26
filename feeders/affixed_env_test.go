package feeders

import (
	"errors"
	"testing"
)

const (
	localhostHost = "localhost"
)

func TestAffixedEnvFeeder(t *testing.T) {
	type Config struct {
		Host     string `env:"HOST"`
		Port     int    `env:"PORT"`
		Username string `env:"USER"`
		Nested   struct {
			Setting bool `env:"SETTING"`
		}
	}

	t.Run("with prefix and suffix", func(t *testing.T) {
		t.Setenv("APP_HOST_TEST", "localhost")
		t.Setenv("APP_PORT_TEST", "8080")
		t.Setenv("APP_USER_TEST", "admin")
		t.Setenv("APP_SETTING_TEST", "true")

		var config Config
		feeder := NewAffixedEnvFeeder("APP", "TEST")
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.Host != localhostHost {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.Host)
		}
		if config.Port != 8080 {
			t.Errorf("Expected Port to be 8080, got %d", config.Port)
		}
		if config.Username != "admin" {
			t.Errorf("Expected Username to be 'admin', got '%s'", config.Username)
		}
		if !config.Nested.Setting {
			t.Errorf("Expected Nested.Setting to be true")
		}
	})

	t.Run("with prefix only", func(t *testing.T) {
		t.Setenv("APP_HOST", "localhost")

		var config Config
		feeder := NewAffixedEnvFeeder("APP", "")
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.Host != localhostHost {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.Host)
		}
	})

	t.Run("with suffix only", func(t *testing.T) {
		t.Setenv("HOST_TEST", "localhost")

		var config Config
		feeder := NewAffixedEnvFeeder("", "TEST")
		err := feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.Host != localhostHost {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.Host)
		}
	})

	t.Run("empty prefix and suffix", func(t *testing.T) {
		var config Config
		feeder := NewAffixedEnvFeeder("", "")
		err := feeder.Feed(&config)

		if !errors.Is(err, ErrEnvEmptyPrefixAndSuffix) {
			t.Fatalf("Expected ErrEnvEmptyPrefixAndSuffix, got %v", err)
		}
	})

	t.Run("invalid structure", func(t *testing.T) {
		feeder := NewAffixedEnvFeeder("APP", "TEST")
		err := feeder.Feed("not a struct pointer")

		if !errors.Is(err, ErrEnvInvalidStructure) {
			t.Fatalf("Expected ErrEnvInvalidStructure, got %v", err)
		}
	})
}
