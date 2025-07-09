package feeders

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnifiedEnvCatalogIntegration(t *testing.T) {
	// Reset catalog
	ResetGlobalEnvCatalog()

	// Create test .env file
	envContent := []byte(`
# Database configuration
DB_HOST=dotenv-host
DB_PORT=5432
DB_USER=dotenv-user

# App configuration  
APP_ENV=development
APP_DEBUG=true
`)

	tempFile := filepath.Join(os.TempDir(), "integration_test.env")
	err := os.WriteFile(tempFile, envContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(tempFile)

	// Set some OS environment variables that should override .env
	t.Setenv("DB_HOST", "os-env-host") // Should override .env value
	t.Setenv("APP_PORT", "8080")       // Only in OS env

	type Config struct {
		Database struct {
			Host string `env:"DB_HOST"`
			Port int    `env:"DB_PORT"`
			User string `env:"DB_USER"`
		}
		App struct {
			Environment string `env:"APP_ENV"`
			Debug       bool   `env:"APP_DEBUG"`
			Port        int    `env:"APP_PORT"`
		}
	}

	var config Config

	// Set up field tracking
	tracker := NewDefaultFieldTracker()

	// Step 1: Load .env file into catalog
	dotEnvFeeder := NewDotEnvFeeder(tempFile)
	dotEnvFeeder.SetFieldTracker(tracker)
	err = dotEnvFeeder.Feed(&config)
	if err != nil {
		t.Fatalf("DotEnvFeeder failed: %v", err)
	}

	// Step 2: Use EnvFeeder to populate remaining fields
	envFeeder := NewEnvFeeder()
	envFeeder.SetFieldTracker(tracker)
	err = envFeeder.Feed(&config)
	if err != nil {
		t.Fatalf("EnvFeeder failed: %v", err)
	}

	// Verify results
	t.Run("verify_os_env_precedence", func(t *testing.T) {
		// DB_HOST should be from OS env (precedence over .env)
		if config.Database.Host != "os-env-host" {
			t.Errorf("Expected DB_HOST='os-env-host' (OS env), got '%s'", config.Database.Host)
		}
	})

	t.Run("verify_dotenv_values", func(t *testing.T) {
		// DB_PORT should be from .env (not in OS env)
		if config.Database.Port != 5432 {
			t.Errorf("Expected DB_PORT=5432 (from .env), got %d", config.Database.Port)
		}

		// DB_USER should be from .env (not in OS env)
		if config.Database.User != "dotenv-user" {
			t.Errorf("Expected DB_USER='dotenv-user' (from .env), got '%s'", config.Database.User)
		}

		// APP_ENV should be from .env (not in OS env)
		if config.App.Environment != "development" {
			t.Errorf("Expected APP_ENV='development' (from .env), got '%s'", config.App.Environment)
		}

		// APP_DEBUG should be from .env (not in OS env)
		if !config.App.Debug {
			t.Errorf("Expected APP_DEBUG=true (from .env), got %v", config.App.Debug)
		}
	})

	t.Run("verify_os_only_values", func(t *testing.T) {
		// APP_PORT should be from OS env (only in OS env)
		if config.App.Port != 8080 {
			t.Errorf("Expected APP_PORT=8080 (OS env only), got %d", config.App.Port)
		}
	})
	t.Run("verify_field_tracking", func(t *testing.T) {
		populations := tracker.GetFieldPopulations()
		if len(populations) == 0 {
			t.Fatal("No field populations recorded")
		}

		sourceMap := make(map[string]string)
		for _, pop := range populations {
			sourceMap[pop.SourceKey] = pop.SourceType
		}

		// Check that we have tracking - all should be "env" since EnvFeeder runs last
		// and reads from the unified catalog (which includes both OS env and .env values)
		foundEnv := false

		for key, sourceType := range sourceMap {
			t.Logf("Field tracking: %s from %s", key, sourceType)
			if sourceType == "env" {
				foundEnv = true
			}
		}

		if !foundEnv {
			t.Error("Expected fields to be tracked as coming from env feeder")
		}

		// Verify that the precedence is working correctly in the final values
		// DB_HOST should be "os-env-host" (OS env precedence over .env)
		if config.Database.Host != "os-env-host" {
			t.Errorf("Expected DB_HOST='os-env-host' (OS env precedence), got '%s'", config.Database.Host)
		}
		// DB_PORT should be 5432 (.env value, not in OS env)
		if config.Database.Port != 5432 {
			t.Errorf("Expected DB_PORT=5432 (.env value), got %d", config.Database.Port)
		}
	})

	t.Run("test_catalog_source_tracking", func(t *testing.T) {
		catalog := GetGlobalEnvCatalog()

		// Test source tracking
		hostSource := catalog.GetSource("DB_HOST")
		portSource := catalog.GetSource("DB_PORT")
		appPortSource := catalog.GetSource("APP_PORT")

		if hostSource != "os_env" {
			t.Errorf("Expected DB_HOST source to be 'os_env', got '%s'", hostSource)
		}

		if portSource != "dotenv:"+tempFile {
			t.Errorf("Expected DB_PORT source to be 'dotenv:...', got '%s'", portSource)
		}

		if appPortSource != "os_env" {
			t.Errorf("Expected APP_PORT source to be 'os_env', got '%s'", appPortSource)
		}
	})
}

func TestMultiFeederCombination(t *testing.T) {
	// Reset catalog
	ResetGlobalEnvCatalog()

	// Create test .env file
	envContent := []byte(`
# Base configuration
BASE_HOST=dotenv-base-host
BASE_PORT=3000
`)

	tempFile := filepath.Join(os.TempDir(), "multi_feeder_test.env")
	err := os.WriteFile(tempFile, envContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}
	defer os.Remove(tempFile)

	// Set OS environment variables
	t.Setenv("OS_OVERRIDE_HOST", "os-host")
	t.Setenv("PROD__CONFIG__ENV", "production") // For AffixedEnvFeeder

	type Config struct {
		BaseHost     string `env:"BASE_HOST"`
		BasePort     int    `env:"BASE_PORT"`
		OverrideHost string `env:"OS_OVERRIDE_HOST"`
		ProdConfig   string `env:"CONFIG"` // For AffixedEnvFeeder
	}

	var config Config
	tracker := NewDefaultFieldTracker()

	// Feed in order: DotEnv → Env → AffixedEnv
	feeders := []interface {
		Feed(interface{}) error
		SetFieldTracker(FieldTracker)
	}{
		NewDotEnvFeeder(tempFile),
		NewEnvFeeder(),
		&AffixedEnvFeeder{Prefix: "PROD_", Suffix: "_ENV"},
	}

	for i, feeder := range feeders {
		feeder.SetFieldTracker(tracker)
		err := feeder.Feed(&config)
		if err != nil {
			t.Fatalf("Feeder %d failed: %v", i, err)
		}
	}

	// Verify final values
	if config.BaseHost != "dotenv-base-host" {
		t.Errorf("Expected BaseHost='dotenv-base-host', got '%s'", config.BaseHost)
	}

	if config.BasePort != 3000 {
		t.Errorf("Expected BasePort=3000, got %d", config.BasePort)
	}

	if config.OverrideHost != "os-host" {
		t.Errorf("Expected OverrideHost='os-host', got '%s'", config.OverrideHost)
	}

	if config.ProdConfig != "production" {
		t.Errorf("Expected ProdConfig='production', got '%s'", config.ProdConfig)
	}

	// Verify field tracking shows correct sources
	populations := tracker.GetFieldPopulations()
	if len(populations) == 0 {
		t.Fatal("No field populations recorded")
	}

	t.Logf("Recorded %d field populations:", len(populations))
	for _, pop := range populations {
		t.Logf("  %s = %v (from %s, key: %s)", pop.FieldPath, pop.Value, pop.SourceType, pop.SourceKey)
	}
}
