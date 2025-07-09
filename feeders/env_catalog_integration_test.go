package feeders

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestEnvCatalogIntegration tests the unified environment catalog with mixed feeders
func TestEnvCatalogIntegration(t *testing.T) {
	// Create a temporary .env file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	envContent := `# Test .env file
APP_NAME=MyApp
APP_VERSION=2.0
DEBUG=true
DATABASE_HOST=localhost
DATABASE_PORT=5432
`
	err := os.WriteFile(envFile, []byte(envContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Test configuration structures
	type DatabaseConfig struct {
		Host string `env:"HOST"`
		Port int    `env:"PORT"`
	}

	type AppConfig struct {
		Name     string `env:"APP_NAME"`
		Version  string `env:"APP_VERSION"`
		Debug    bool   `env:"DEBUG"`
		Database DatabaseConfig
	}

	t.Run("DotEnv + EnvFeeder integration", func(t *testing.T) {
		// Reset global catalog for clean test
		ResetGlobalEnvCatalog()

		// Set some OS environment variables that will override .env
		t.Setenv("APP_VERSION", "3.0") // This should override .env value

		var config AppConfig

		// First, load .env file using DotEnvFeeder
		dotEnvFeeder := NewDotEnvFeeder(envFile)
		tracker := NewDefaultFieldTracker()
		dotEnvFeeder.SetFieldTracker(tracker)

		err := dotEnvFeeder.Feed(&config)
		if err != nil {
			t.Fatalf("DotEnvFeeder failed: %v", err)
		}

		// Verify .env values were loaded
		if config.Name != "MyApp" {
			t.Errorf("Expected Name 'MyApp', got '%s'", config.Name)
		}
		if config.Version != "3.0" { // OS env should override .env
			t.Errorf("Expected Version '3.0' (OS override), got '%s'", config.Version)
		}
		if !config.Debug {
			t.Errorf("Expected Debug true, got %v", config.Debug)
		}

		// Now use EnvFeeder to populate additional fields not handled by DotEnv
		envFeeder := NewEnvFeeder()
		envFeeder.SetFieldTracker(tracker)

		// This should work because .env values are now in the global catalog
		err = envFeeder.Feed(&config)
		if err != nil {
			t.Fatalf("EnvFeeder failed: %v", err)
		}

		// Verify field tracking captured both sources
		populations := tracker.GetFieldPopulations()
		foundSources := make(map[string]string)
		for _, pop := range populations {
			foundSources[pop.FieldPath] = pop.SourceType
		}

		// Should have entries from both feeders
		if len(populations) < 3 {
			t.Errorf("Expected at least 3 field populations, got %d", len(populations))
		}
	})

	t.Run("DotEnv + AffixedEnvFeeder integration", func(t *testing.T) {
		// Reset global catalog
		ResetGlobalEnvCatalog()

		// Set up environment variables with prefix/suffix pattern
		// AffixedEnvFeeder with prefix "PROD_" and suffix "_ENV"
		// constructs: prefix + "_" + envTag + "_" + suffix
		// For env:"HOST" -> "PROD_" + "_" + "HOST" + "_" + "_ENV" = "PROD__HOST__ENV"
		t.Setenv("PROD__HOST__ENV", "prod.example.com")
		t.Setenv("PROD__PORT__ENV", "3306")

		var config AppConfig

		// Load .env first
		dotEnvFeeder := NewDotEnvFeeder(envFile)
		err := dotEnvFeeder.Feed(&config)
		if err != nil {
			t.Fatalf("DotEnvFeeder failed: %v", err)
		}

		// Now use AffixedEnvFeeder for database config
		affixedFeeder := NewAffixedEnvFeeder("PROD_", "_ENV")
		tracker := NewDefaultFieldTracker()
		affixedFeeder.SetFieldTracker(tracker)

		err = affixedFeeder.Feed(&config.Database)
		if err != nil {
			t.Fatalf("AffixedEnvFeeder failed: %v", err)
		}

		// Verify values from both sources
		if config.Name != "MyApp" { // From .env
			t.Errorf("Expected Name 'MyApp', got '%s'", config.Name)
		}
		if config.Database.Host != "prod.example.com" { // From affixed env
			t.Errorf("Expected Database.Host 'prod.example.com', got '%s'", config.Database.Host)
		}
		if config.Database.Port != 3306 { // From affixed env
			t.Errorf("Expected Database.Port 3306, got %d", config.Database.Port)
		}

		// Verify field tracking
		populations := tracker.GetFieldPopulations()
		if len(populations) != 2 {
			t.Errorf("Expected 2 field populations for database, got %d", len(populations))
		}

		// Check that source tracking is correct
		for _, pop := range populations {
			if pop.SourceType != "env_affixed" {
				t.Errorf("Expected source type 'env_affixed', got '%s'", pop.SourceType)
			}
		}
	})

	t.Run("Feeder evaluation order", func(t *testing.T) {
		// Reset global catalog
		ResetGlobalEnvCatalog()

		// Test that OS environment takes precedence over .env
		t.Setenv("APP_NAME", "OSOverride")

		var config AppConfig

		// Load .env first
		dotEnvFeeder := NewDotEnvFeeder(envFile)
		err := dotEnvFeeder.Feed(&config)
		if err != nil {
			t.Fatalf("DotEnvFeeder failed: %v", err)
		}

		// OS env should take precedence
		if config.Name != "OSOverride" {
			t.Errorf("Expected Name 'OSOverride' (OS precedence), got '%s'", config.Name)
		}

		// But .env values should still be available for other fields
		if config.Version != "2.0" {
			t.Errorf("Expected Version '2.0' (from .env), got '%s'", config.Version)
		}
	})

	t.Run("Catalog source tracking", func(t *testing.T) {
		// Reset global catalog
		ResetGlobalEnvCatalog()

		catalog := GetGlobalEnvCatalog()

		// Load .env file
		err := catalog.LoadFromDotEnv(envFile)
		if err != nil {
			t.Fatalf("Failed to load .env into catalog: %v", err)
		}

		// Set OS env var
		t.Setenv("TEST_OS_VAR", "os_value")

		// Check sources
		envSource := catalog.GetSource("APP_NAME")
		if envSource != fmt.Sprintf("dotenv:%s", envFile) {
			t.Errorf("Expected source 'dotenv:%s', got '%s'", envFile, envSource)
		}

		// Get OS var (should be detected and cached)
		osValue, exists := catalog.Get("TEST_OS_VAR")
		if !exists || osValue != "os_value" {
			t.Errorf("Expected OS var 'os_value', got '%s', exists: %v", osValue, exists)
		}

		osSource := catalog.GetSource("TEST_OS_VAR")
		if osSource != "os_env" {
			t.Errorf("Expected source 'os_env', got '%s'", osSource)
		}
	})
}
