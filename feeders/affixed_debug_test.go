package feeders

import (
	"testing"
)

func TestAffixedEnvFeederCatalogDebug(t *testing.T) {
	// Reset global catalog
	ResetGlobalEnvCatalog()

	// Set environment variables (double underscores per AffixedEnvFeeder pattern)
	t.Setenv("PROD__HOST__ENV", "prod.example.com")
	t.Setenv("PROD__PORT__ENV", "3306")

	type DatabaseConfig struct {
		Host string `env:"HOST"`
		Port int    `env:"PORT"`
	}

	var config DatabaseConfig

	// Create and test AffixedEnvFeeder
	feeder := NewAffixedEnvFeeder("PROD_", "_ENV")
	tracker := NewDefaultFieldTracker()
	feeder.SetFieldTracker(tracker)

	// Enable verbose debug
	logger := &TestLogger2{}
	feeder.SetVerboseDebug(true, logger)

	err := feeder.Feed(&config)
	if err != nil {
		t.Fatalf("AffixedEnvFeeder failed: %v", err)
	}

	t.Logf("Config after feeding: Host='%s', Port=%d", config.Host, config.Port)

	// Check field tracking
	populations := tracker.GetFieldPopulations()
	t.Logf("Field populations: %d", len(populations))
	for _, pop := range populations {
		t.Logf("  Field: %s, Value: %v, SourceKey: %s, SourceType: %s",
			pop.FieldPath, pop.Value, pop.SourceKey, pop.SourceType)
	}

	// Verify results
	if config.Host != "prod.example.com" {
		t.Errorf("Expected Host 'prod.example.com', got '%s'", config.Host)
	}
	if config.Port != 3306 {
		t.Errorf("Expected Port 3306, got %d", config.Port)
	}
}

type TestLogger2 struct{}

func (l *TestLogger2) Debug(msg string, args ...any) {
	// Just for debug output
}
