package modular

import (
	"os"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
)

// TestIssueReproduction demonstrates the exact scenario from the GitHub issue
// where environment variables would override explicit YAML configuration,
// breaking test isolation. This test now passes with the priority system.
func TestIssueReproduction(t *testing.T) {
	type TestConfig struct {
		SDKKey string `yaml:"sdkKey" env:"SDK_KEY"`
	}

	t.Run("original_issue_environment_overrides_yaml", func(t *testing.T) {
		// Set up environment variable (simulating host environment)
		t.Setenv("SDK_KEY", "env-value")

		// Create YAML config with explicit value
		yamlContent := `sdkKey: "yaml-value"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		// Configure feeders: YAML first, then Env (OLD BEHAVIOR - no priority)
		cfg := &TestConfig{}
		config := NewConfig()
		config.AddFeeder(feeders.NewYamlFeeder(yamlPath))
		config.AddFeeder(feeders.NewEnvFeeder())
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// OLD BEHAVIOR: Env var overrode YAML (this was the problem)
		if cfg.SDKKey != "env-value" {
			t.Errorf("Without priority, expected env to override YAML but got '%s'", cfg.SDKKey)
		}
		t.Logf("✗ Without priority: SDKKey = '%s' (env overrode explicit YAML config)", cfg.SDKKey)
	})

	t.Run("fixed_with_priority_yaml_overrides_env", func(t *testing.T) {
		// Set up environment variable (simulating host environment)
		t.Setenv("SDK_KEY", "env-value")

		// Create YAML config with explicit value
		yamlContent := `sdkKey: "yaml-value"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		// Configure feeders WITH PRIORITY: YAML has higher priority
		cfg := &TestConfig{}
		config := NewConfig()
		config.AddFeeder(feeders.NewEnvFeeder().WithPriority(50))           // Lower priority
		config.AddFeeder(feeders.NewYamlFeeder(yamlPath).WithPriority(100)) // Higher priority
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// NEW BEHAVIOR: YAML overrides env because of higher priority (FIXED!)
		if cfg.SDKKey != "yaml-value" {
			t.Errorf("Expected YAML value 'yaml-value', got '%s' - priority control failed", cfg.SDKKey)
		}
		t.Logf("✓ With priority: SDKKey = '%s' (YAML overrode env as intended)", cfg.SDKKey)
	})

	t.Run("production_pattern_env_overrides_yaml", func(t *testing.T) {
		// Set up environment variable (production override)
		t.Setenv("SDK_KEY", "production-key")

		// Create YAML config with default value
		yamlContent := `sdkKey: "default-key"`
		yamlPath := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Configure feeders WITH PRIORITY: Env has higher priority
		cfg := &TestConfig{}
		config := NewConfig()
		config.AddFeeder(feeders.NewYamlFeeder(yamlPath).WithPriority(50)) // Lower priority - defaults
		config.AddFeeder(feeders.NewEnvFeeder().WithPriority(100))         // Higher priority - overrides
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// Production pattern: Env overrides YAML defaults
		if cfg.SDKKey != "production-key" {
			t.Errorf("Expected production value 'production-key', got '%s'", cfg.SDKKey)
		}
		t.Logf("✓ Production pattern: SDKKey = '%s' (env overrode YAML defaults)", cfg.SDKKey)
	})
}
