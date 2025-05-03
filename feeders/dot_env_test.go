package feeders

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotEnvFeeder(t *testing.T) {
	// Create a temporary .env file
	envContent := []byte(`
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=secret
`)

	// Use os.TempDir() for OS-agnostic temp directory
	tempFile := filepath.Join(os.TempDir(), "test.env")
	err := os.WriteFile(tempFile, envContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(tempFile)

	t.Run("read from .env file", func(t *testing.T) {
		type Config struct {
			DB struct {
				Host     string `env:"DB_HOST"`
				Port     int    `env:"DB_PORT"`
				User     string `env:"DB_USER"`
				Password string `env:"DB_PASS"`
			}
		}

		var config Config
		feeder := NewDotEnvFeeder(tempFile)
		err = feeder.Feed(&config)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if config.DB.Host != "localhost" {
			t.Errorf("Expected Host to be 'localhost', got '%s'", config.DB.Host)
		}
		if config.DB.Port != 5432 {
			t.Errorf("Expected Port to be 5432, got %d", config.DB.Port)
		}
		if config.DB.User != "postgres" {
			t.Errorf("Expected User to be 'postgres', got '%s'", config.DB.User)
		}
		if config.DB.Password != "secret" {
			t.Errorf("Expected Password to be 'secret', got '%s'", config.DB.Password)
		}
	})

	t.Run("non-existent .env file", func(t *testing.T) {
		var config struct{}
		// Also use filepath.Join for the nonexistent file
		feeder := NewDotEnvFeeder(filepath.Join(os.TempDir(), "nonexistent.env"))
		err = feeder.Feed(&config)

		if err == nil {
			t.Fatal("Expected error for non-existent file, got nil")
		}
	})
}
