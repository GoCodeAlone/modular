package feeders

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotEnvFeederDebug(t *testing.T) {
	// Create test .env file
	envContent := []byte(`
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASS=secret
`)

	tempFile := filepath.Join(os.TempDir(), "debug_test.env")
	err := os.WriteFile(tempFile, envContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(tempFile)

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

	// Enable debug logging
	logger := &TestLogger2{}
	feeder.SetVerboseDebug(true, logger)

	err = feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Feed failed: %v", err)
	}

	t.Logf("Config after feeding:")
	t.Logf("  DB.Host: '%s'", config.DB.Host)
	t.Logf("  DB.Port: %d", config.DB.Port)
	t.Logf("  DB.User: '%s'", config.DB.User)
	t.Logf("  DB.Password: '%s'", config.DB.Password)

	// Test direct catalog access
	catalog := GetGlobalEnvCatalog()
	value, exists := catalog.Get("DB_HOST")
	t.Logf("Catalog DB_HOST: value='%s', exists=%v", value, exists)

	// Test OS access
	osValue := os.Getenv("DB_HOST")
	t.Logf("OS DB_HOST: '%s'", osValue)
}
