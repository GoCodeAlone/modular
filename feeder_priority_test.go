package modular

import (
	"os"
	"testing"

	"github.com/GoCodeAlone/modular/feeders"
)

// TestFeederPriorityBasic tests the basic priority functionality
func TestFeederPriorityBasic(t *testing.T) {
	type TestConfig struct {
		SDKKey string `yaml:"sdkKey" json:"sdkKey" env:"SDK_KEY"`
		Host   string `yaml:"host" json:"host" env:"HOST"`
	}

	t.Run("yaml_higher_priority_overrides_env", func(t *testing.T) {
		// Set up environment variable
		t.Setenv("SDK_KEY", "env-value")
		t.Setenv("HOST", "env-host")

		// Create YAML config with explicit values
		yamlContent := `sdkKey: "yaml-value"
host: "yaml-host"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders: Env first with low priority, YAML second with high priority
		testFeeders := []Feeder{
			feeders.NewEnvFeeder().WithPriority(50),           // Low priority
			feeders.NewYamlFeeder(yamlPath).WithPriority(100), // High priority - should win
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// YAML should override env because it has higher priority
		if cfg.SDKKey != "yaml-value" {
			t.Errorf("Expected YAML value 'yaml-value', got '%s' - env var overrode despite lower priority", cfg.SDKKey)
		}
		if cfg.Host != "yaml-host" {
			t.Errorf("Expected YAML host 'yaml-host', got '%s'", cfg.Host)
		}
	})

	t.Run("env_higher_priority_overrides_yaml", func(t *testing.T) {
		// Set up environment variable
		t.Setenv("SDK_KEY", "env-value")
		t.Setenv("HOST", "env-host")

		// Create YAML config with explicit values
		yamlContent := `sdkKey: "yaml-value"
host: "yaml-host"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders: YAML first with low priority, Env second with high priority
		testFeeders := []Feeder{
			feeders.NewYamlFeeder(yamlPath).WithPriority(50), // Low priority
			feeders.NewEnvFeeder().WithPriority(100),         // High priority - should win
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// Env should override YAML because it has higher priority
		if cfg.SDKKey != "env-value" {
			t.Errorf("Expected env value 'env-value', got '%s'", cfg.SDKKey)
		}
		if cfg.Host != "env-host" {
			t.Errorf("Expected env host 'env-host', got '%s'", cfg.Host)
		}
	})

	t.Run("default_priority_maintains_order", func(t *testing.T) {
		// Set up environment variable
		t.Setenv("SDK_KEY", "env-value")

		// Create YAML config
		yamlContent := `sdkKey: "yaml-value"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders with default priority (0) - should maintain order
		testFeeders := []Feeder{
			feeders.NewYamlFeeder(yamlPath), // Applied first
			feeders.NewEnvFeeder(),          // Applied second - should override
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// With default priority (both 0), later feeder (Env) should win
		if cfg.SDKKey != "env-value" {
			t.Errorf("Expected env value 'env-value', got '%s' - default priority should maintain order", cfg.SDKKey)
		}
	})

	t.Run("multiple_priorities_sorted_correctly", func(t *testing.T) {
		// Set up environment variables
		t.Setenv("SDK_KEY", "env-value")

		// Create YAML config
		yamlContent := `sdkKey: "yaml-value"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Create JSON config
		jsonContent := `{"sdkKey": "json-value"}`
		jsonPath := t.TempDir() + "/test-config.json"
		if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
			t.Fatalf("Failed to write JSON file: %v", err)
		}

		// Configure feeders with different priorities
		testFeeders := []Feeder{
			feeders.NewEnvFeeder().WithPriority(10),           // Lowest priority
			feeders.NewJSONFeeder(jsonPath).WithPriority(200), // Highest priority - should win
			feeders.NewYamlFeeder(yamlPath).WithPriority(100), // Medium priority
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// JSON has highest priority, should win
		if cfg.SDKKey != "json-value" {
			t.Errorf("Expected JSON value 'json-value', got '%s'. SDKKey value suggests wrong feeder won", cfg.SDKKey)
			t.Logf("Config state: SDKKey=%s, Host=%s", cfg.SDKKey, cfg.Host)
		}
	})
}

// TestFeederPriorityIsolatesTests validates the test isolation use case from the issue
func TestFeederPriorityIsolatesTests(t *testing.T) {
	type TestConfig struct {
		SDKKey string `yaml:"sdkKey" env:"SDK_KEY"`
	}

	// This is the exact scenario from the issue: environment variable exists,
	// but test wants to use explicit YAML config
	t.Run("test_isolation_with_env_var_present", func(t *testing.T) {
		// Set up environment variable (simulating production environment)
		t.Setenv("SDK_KEY", "production-sdk-key")

		// Create test YAML config with specific value
		yamlContent := `sdkKey: "test-sdk-key"`
		yamlPath := t.TempDir() + "/test-config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders: YAML with higher priority for test isolation
		testFeeders := []Feeder{
			feeders.NewEnvFeeder().WithPriority(50),           // Lower priority
			feeders.NewYamlFeeder(yamlPath).WithPriority(100), // Higher priority - wins
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// Test should get explicit YAML value, not environment variable
		if cfg.SDKKey != "test-sdk-key" {
			t.Errorf("Test isolation failed: Expected test value 'test-sdk-key', got '%s' from environment", cfg.SDKKey)
		}
	})

	t.Run("production_env_overrides_defaults", func(t *testing.T) {
		// Set up environment variable
		t.Setenv("SDK_KEY", "production-sdk-key")

		// Create default YAML config
		yamlContent := `sdkKey: "default-sdk-key"`
		yamlPath := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders: Env with higher priority for production overrides
		testFeeders := []Feeder{
			feeders.NewYamlFeeder(yamlPath).WithPriority(50), // Lower priority - defaults
			feeders.NewEnvFeeder().WithPriority(100),         // Higher priority - production override
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// Production environment should override defaults
		if cfg.SDKKey != "production-sdk-key" {
			t.Errorf("Expected production value 'production-sdk-key', got '%s'", cfg.SDKKey)
		}
	})
}

// TestFeederPriorityBackwardCompatibility ensures default behavior is preserved
func TestFeederPriorityBackwardCompatibility(t *testing.T) {
	type TestConfig struct {
		Value string `yaml:"value" env:"VALUE"`
	}

	t.Run("no_priority_specified_uses_order", func(t *testing.T) {
		// Set up environment variable
		t.Setenv("VALUE", "from-env")

		// Create YAML config
		yamlContent := `value: "from-yaml"`
		yamlPath := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Configure feeders WITHOUT priority - should use order
		testFeeders := []Feeder{
			feeders.NewYamlFeeder(yamlPath), // First
			feeders.NewEnvFeeder(),          // Second - should override
		}

		// Create config provider and feed
		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// Without priority, later feeder wins (backward compatible behavior)
		if cfg.Value != "from-env" {
			t.Errorf("Backward compatibility broken: Expected 'from-env', got '%s'", cfg.Value)
		}
	})
}

// TestAffixedEnvFeederPriority tests the AffixedEnvFeeder priority functionality
func TestAffixedEnvFeederPriority(t *testing.T) {
	type TestConfig struct {
		APIKey string `env:"API_KEY"`
	}

	t.Run("affixed_env_feeder_with_priority", func(t *testing.T) {
		// Set up environment variable with prefix
		t.Setenv("PREFIX_API_KEY", "prefixed-value")
		t.Setenv("API_KEY", "plain-value")

		// Create YAML config
		yamlContent := `apiKey: "yaml-value"`
		yamlPath := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Test 1: AffixedEnvFeeder with higher priority should win
		testFeeders := []Feeder{
			feeders.NewYamlFeeder(yamlPath).WithPriority(50),             // Lower priority
			feeders.NewAffixedEnvFeeder("PREFIX_", "").WithPriority(100), // Higher priority - should win
		}

		cfg := &TestConfig{}
		config := NewConfig()
		for _, feeder := range testFeeders {
			config.AddFeeder(feeder)
		}
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// AffixedEnvFeeder should win due to higher priority
		if cfg.APIKey != "prefixed-value" {
			t.Errorf("Expected AffixedEnvFeeder value 'prefixed-value', got '%s'", cfg.APIKey)
		}
	})

	t.Run("affixed_env_feeder_builder_pattern", func(t *testing.T) {
		// Test that the builder pattern works correctly
		t.Setenv("PREFIX_API_KEY", "builder-test")

		// This is the proper way to use WithPriority with AffixedEnvFeeder
		feeder := feeders.NewAffixedEnvFeeder("PREFIX_", "").WithPriority(100)

		// Verify priority was set
		prioritized, ok := any(feeder).(PrioritizedFeeder)
		if !ok {
			t.Fatal("AffixedEnvFeeder does not implement PrioritizedFeeder interface")
		}

		if prioritized.Priority() != 100 {
			t.Errorf("Expected priority 100, got %d", prioritized.Priority())
		}

		// Test feeding
		cfg := &TestConfig{}
		if err := feeder.Feed(cfg); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		if cfg.APIKey != "builder-test" {
			t.Errorf("Expected 'builder-test', got '%s'", cfg.APIKey)
		}
	})

	t.Run("affixed_env_feeder_chaining", func(t *testing.T) {
		// Test that chaining works as expected - this is the correct usage pattern
		t.Setenv("TEST_API_KEY_PROD", "chained-value")

		cfg := &TestConfig{}
		config := NewConfig()

		// Correct: Chain directly when adding to config
		config.AddFeeder(feeders.NewAffixedEnvFeeder("TEST_", "_PROD").WithPriority(200))
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		if cfg.APIKey != "chained-value" {
			t.Errorf("Expected 'chained-value', got '%s'", cfg.APIKey)
		}
	})
}

// TestTenantAffixedEnvFeederPriority tests the TenantAffixedEnvFeeder priority functionality
func TestTenantAffixedEnvFeederPriority(t *testing.T) {
	type TestConfig struct {
		APIKey string `env:"API_KEY"`
	}

	t.Run("tenant_affixed_env_feeder_with_priority", func(t *testing.T) {
		// Set up environment variable with tenant prefix
		t.Setenv("TENANT1_API_KEY", "tenant1-value")
		t.Setenv("API_KEY", "plain-value")

		// Create YAML config
		yamlContent := `apiKey: "yaml-value"`
		yamlPath := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write YAML file: %v", err)
		}

		// Create tenant affixed feeder that adds "TENANT1_" prefix
		tenantFeeder := feeders.NewTenantAffixedEnvFeeder(
			func(tenantID string) string { return "TENANT1_" },
			func(tenantID string) string { return "" },
		).WithPriority(100)

		// Set the prefix before using
		tenantFeeder.SetPrefixFunc("tenant1")

		// Test priority was set correctly
		prioritized, ok := any(tenantFeeder).(PrioritizedFeeder)
		if !ok {
			t.Fatal("TenantAffixedEnvFeeder does not implement PrioritizedFeeder interface")
		}

		if prioritized.Priority() != 100 {
			t.Errorf("Expected priority 100, got %d", prioritized.Priority())
		}

		// Test with config
		cfg := &TestConfig{}
		config := NewConfig()
		config.AddFeeder(feeders.NewYamlFeeder(yamlPath).WithPriority(50))
		config.AddFeeder(tenantFeeder)
		config.AddStructKey("_main", cfg)

		if err := config.Feed(); err != nil {
			t.Fatalf("Failed to feed config: %v", err)
		}

		// TenantAffixedEnvFeeder should win due to higher priority
		if cfg.APIKey != "tenant1-value" {
			t.Errorf("Expected TenantAffixedEnvFeeder value 'tenant1-value', got '%s'", cfg.APIKey)
		}
	})

	t.Run("tenant_affixed_env_feeder_priority_chaining", func(t *testing.T) {
		// Verify that WithPriority returns the same feeder for chaining
		feeder := feeders.NewTenantAffixedEnvFeeder(
			func(tenantID string) string { return "PREFIX_" },
			func(tenantID string) string { return "" },
		)

		chainedFeeder := feeder.WithPriority(200)

		if chainedFeeder != feeder {
			t.Error("WithPriority should return the same feeder instance for chaining")
		}

		if chainedFeeder.Priority() != 200 {
			t.Errorf("Expected priority 200, got %d", chainedFeeder.Priority())
		}
	})
}
