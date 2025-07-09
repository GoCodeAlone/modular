package feeders

import (
	"fmt"
	"os"
	"testing"
)

// TestConfig struct for TenantAffixedEnv field tracking tests
type TestTenantAffixedEnvConfig struct {
	Name    string `env:"NAME"`
	Port    int    `env:"PORT"`
	Enabled bool   `env:"ENABLED"`
}

func TestTenantAffixedEnvFeeder_FieldTracking(t *testing.T) {
	// Set up environment variables for tenant "test123"
	// The AffixedEnvFeeder converts prefix/suffix to uppercase and constructs env vars as:
	// ToUpper(prefix) + "_" + ToUpper(envTag) + "_" + ToUpper(suffix)
	// With prefix "APP_test123_" -> "APP_TEST123_" and suffix "_PROD" -> "_PROD":
	// APP_TEST123_ + _ + NAME + _ + _PROD = APP_TEST123__NAME__PROD
	envVars := map[string]string{
		"APP_TEST123__NAME__PROD":    "tenant-app",
		"APP_TEST123__PORT__PROD":    "9090",
		"APP_TEST123__ENABLED__PROD": "true",
		"OTHER_VAR":                  "ignored", // Should not be matched
	}

	// Set environment variables for test
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// Clean up after test
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Create tenant affixed feeder
	// Prefix function: "APP_" + tenantId + "_" = "APP_test123_"
	// Suffix function: "_" + environment = "_PROD"
	prefixFunc := func(tenantId string) string {
		return "APP_" + tenantId + "_"
	}
	suffixFunc := func(environment string) string {
		return "_" + environment
	}

	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)

	// Set tenant and environment
	feeder.SetPrefixFunc("test123")
	feeder.SetSuffixFunc("PROD")

	// Set field tracker
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	// Test config structure
	var config TestTenantAffixedEnvConfig

	// Feed the configuration
	err := feeder.Feed(&config)
	if err != nil {
		t.Fatalf("Failed to feed config: %v", err)
	}

	// Verify configuration was populated correctly
	if config.Name != "tenant-app" {
		t.Errorf("Expected Name to be 'tenant-app', got %s", config.Name)
	}
	if config.Port != 9090 {
		t.Errorf("Expected Port to be 9090, got %d", config.Port)
	}
	if !config.Enabled {
		t.Errorf("Expected Enabled to be true, got %v", config.Enabled)
	}

	// Get field populations
	populations := tracker.GetFieldPopulations()

	// Verify we have tracking information for all fields
	expectedFields := []string{"Name", "Port", "Enabled"}

	for _, fieldPath := range expectedFields {
		found := false
		for _, pop := range populations {
			if pop.FieldPath == fieldPath {
				found = true
				// Verify basic tracking information
				if pop.FeederType != "AffixedEnvFeeder" { // Since it delegates to AffixedEnvFeeder
					t.Errorf("Expected FeederType 'AffixedEnvFeeder' for field %s, got %s", fieldPath, pop.FeederType)
				}
				if pop.SourceType != "env_affixed" {
					t.Errorf("Expected SourceType 'env_affixed' for field %s, got %s", fieldPath, pop.SourceType)
				}
				if pop.SourceKey == "" {
					t.Errorf("Expected non-empty SourceKey for field %s", fieldPath)
				}
				if pop.Value == nil {
					t.Errorf("Expected non-nil Value for field %s", fieldPath)
				}
				break
			}
		}
		if !found {
			t.Errorf("Field tracking not found for field: %s", fieldPath)
		}
	}

	// Verify specific field values and source keys in tracking
	for _, pop := range populations {
		switch pop.FieldPath {
		case "Name":
			if fmt.Sprintf("%v", pop.Value) != "tenant-app" {
				t.Errorf("Expected tracked value 'tenant-app' for Name, got %v", pop.Value)
			}
			if pop.SourceKey != "APP_TEST123__NAME__PROD" {
				t.Errorf("Expected SourceKey 'APP_TEST123__NAME__PROD' for Name, got %s", pop.SourceKey)
			}
		case "Port":
			if fmt.Sprintf("%v", pop.Value) != "9090" {
				t.Errorf("Expected tracked value '9090' for Port, got %v", pop.Value)
			}
			if pop.SourceKey != "APP_TEST123__PORT__PROD" {
				t.Errorf("Expected SourceKey 'APP_TEST123__PORT__PROD' for Port, got %s", pop.SourceKey)
			}
		case "Enabled":
			if fmt.Sprintf("%v", pop.Value) != "true" {
				t.Errorf("Expected tracked value 'true' for Enabled, got %v", pop.Value)
			}
			if pop.SourceKey != "APP_TEST123__ENABLED__PROD" {
				t.Errorf("Expected SourceKey 'APP_TEST123__ENABLED__PROD' for Enabled, got %s", pop.SourceKey)
			}
		}
	}
}

func TestTenantAffixedEnvFeeder_SetFieldTracker(t *testing.T) {
	prefixFunc := func(tenantId string) string { return "PREFIX_" }
	suffixFunc := func(environment string) string { return "_SUFFIX" }
	feeder := NewTenantAffixedEnvFeeder(prefixFunc, suffixFunc)
	tracker := NewDefaultFieldTracker()

	// Test that SetFieldTracker method exists and can be called
	feeder.SetFieldTracker(tracker)

	// The actual tracking functionality is tested in TestTenantAffixedEnvFeeder_FieldTracking
}
