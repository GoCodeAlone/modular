package modular

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular/feeders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test config structs
type ValidationTestConfig struct {
	Name        string            `yaml:"name" default:"Default Name" desc:"Name of the config"`
	Port        int               `yaml:"port" default:"8080" required:"true" desc:"Port to listen on"`
	Debug       bool              `yaml:"debug" default:"false" desc:"Enable debug mode"`
	Tags        []string          `yaml:"tags" default:"[\"tag1\", \"tag2\"]" desc:"List of tags"`
	Environment string            `yaml:"environment" required:"true" desc:"Environment (dev, test, prod)"`
	Options     map[string]string `yaml:"options" default:"{\"key1\":\"value1\", \"key2\":\"value2\"}" desc:"Options"`
	NestedCfg   *NestedTestConfig `yaml:"nested" desc:"Nested configuration"`
}

type NestedTestConfig struct {
	Enabled bool   `yaml:"enabled" default:"true" desc:"Enable the nested feature"`
	Timeout int    `yaml:"timeout" default:"30" desc:"Timeout in seconds"`
	APIKey  string `yaml:"apiKey" required:"true" desc:"API key for authentication"`
}

// DurationTestConfig for testing time.Duration default values
type DurationTestConfig struct {
	RequestTimeout time.Duration `yaml:"request_timeout" default:"30s" desc:"Request timeout duration"`
	CacheTTL       time.Duration `yaml:"cache_ttl" default:"5m" desc:"Cache TTL duration"`
	HealthInterval time.Duration `yaml:"health_interval" default:"1h30m" desc:"Health check interval"`
	NoDefault      time.Duration `yaml:"no_default" desc:"Duration with no default"`
	RequiredDur    time.Duration `yaml:"required_dur" required:"true" desc:"Required duration field"`
}

// Implement ConfigValidator
func (c *ValidationTestConfig) Validate() error {
	if c.Port < 1024 && c.Port != 0 {
		return ErrConfigValidationFailed
	}
	// Additional validation could be done here
	return nil
}

func TestProcessConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name: "basic defaults",
			cfg:  &ValidationTestConfig{},
			expected: &ValidationTestConfig{
				Name:        "Default Name",
				Port:        8080,
				Debug:       false,
				Tags:        []string{"tag1", "tag2"},
				Environment: "",
				Options:     map[string]string{"key1": "value1", "key2": "value2"},
				NestedCfg:   nil,
			},
			wantErr: false,
		},
		{
			name: "with values already set",
			cfg: &ValidationTestConfig{
				Name: "Custom Name",
				Port: 9000,
			},
			expected: &ValidationTestConfig{
				Name:        "Custom Name", // Not overwritten
				Port:        9000,          // Not overwritten
				Debug:       false,
				Tags:        []string{"tag1", "tag2"},
				Environment: "",
				Options:     map[string]string{"key1": "value1", "key2": "value2"},
				NestedCfg:   nil,
			},
			wantErr: false,
		},
		{
			name:     "nil config",
			cfg:      nil,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "non-pointer",
			cfg:      ValidationTestConfig{},
			expected: ValidationTestConfig{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ProcessConfigDefaults(tc.cfg)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, tc.cfg)
		})
	}
}

func TestValidateConfigRequired(t *testing.T) {
	tests := []struct {
		name     string
		cfg      interface{}
		wantErr  bool
		errorMsg string
	}{
		{
			name: "all required fields present",
			cfg: &ValidationTestConfig{
				Port:        8080,
				Environment: "dev",
				NestedCfg: &NestedTestConfig{
					APIKey: "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "missing environment",
			cfg: &ValidationTestConfig{
				Port: 8080,
				NestedCfg: &NestedTestConfig{
					APIKey: "test-key",
				},
			},
			wantErr:  true,
			errorMsg: "Environment",
		},
		{
			name: "missing nested api key",
			cfg: &ValidationTestConfig{
				Port:        8080,
				Environment: "dev",
				NestedCfg:   &NestedTestConfig{},
			},
			wantErr:  true,
			errorMsg: "APIKey",
		},
		{
			name: "missing port",
			cfg: &ValidationTestConfig{
				Environment: "dev",
				NestedCfg: &NestedTestConfig{
					APIKey: "test-key",
				},
			},
			wantErr:  true,
			errorMsg: "Port",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfigRequired(tc.cfg)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     interface{}
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &ValidationTestConfig{
				Port:        8080,
				Environment: "dev",
				NestedCfg: &NestedTestConfig{
					APIKey: "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "validation error - port too low",
			cfg: &ValidationTestConfig{
				Port:        80, // Low privileged port, custom validation should fail
				Environment: "dev",
				NestedCfg: &NestedTestConfig{
					APIKey: "test-key",
				},
			},
			wantErr: true,
		},
		{
			name: "missing required field",
			cfg: &ValidationTestConfig{
				Port: 8080,
				// Missing Environment
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(tc.cfg)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestGenerateSampleConfig(t *testing.T) {
	cfg := &ValidationTestConfig{}

	// Test YAML generation
	yamlData, err := GenerateSampleConfig(cfg, "yaml")
	require.NoError(t, err)
	assert.Contains(t, string(yamlData), "name: Default Name")
	assert.Contains(t, string(yamlData), "port: 8080")

	// Test JSON generation
	jsonData, err := GenerateSampleConfig(cfg, "json")
	require.NoError(t, err)
	var jsonCfg map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonCfg)
	require.NoError(t, err)
	assert.Equal(t, "Default Name", jsonCfg["name"])
	assert.InEpsilon(t, 8080, jsonCfg["port"], 0.0001) // Use InEpsilon for float comparison

	// Test TOML generation
	tomlData, err := GenerateSampleConfig(cfg, "toml")
	require.NoError(t, err)
	// TOML field names might be capitalized based on the struct field names
	// so use case-insensitive contains check
	tomlContent := string(tomlData)
	assert.Contains(t, strings.ToLower(tomlContent), strings.ToLower("Name = \"Default Name\""))
	assert.Contains(t, strings.ToLower(tomlContent), strings.ToLower("Port = 8080"))

	// Test invalid format
	_, err = GenerateSampleConfig(cfg, "invalid")
	require.Error(t, err)
}

func TestSaveSampleConfig(t *testing.T) {
	cfg := &ValidationTestConfig{}
	tempFile := os.TempDir() + "/sample_config.yaml"
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			// Log error but don't fail the test
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	err := SaveSampleConfig(cfg, "yaml", tempFile)
	require.NoError(t, err)

	// Verify file exists and contains expected content
	fileData, err := os.ReadFile(tempFile) // #nosec G304 -- reading test-created temp file
	require.NoError(t, err)
	assert.Contains(t, string(fileData), "name: Default Name")
	assert.Contains(t, string(fileData), "port: 8080")
}

func TestProcessConfigDefaults_TimeDuration(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *DurationTestConfig
		expected *DurationTestConfig
		wantErr  bool
	}{
		{
			name: "all duration defaults applied",
			cfg:  &DurationTestConfig{},
			expected: &DurationTestConfig{
				RequestTimeout: 30 * time.Second,
				CacheTTL:       5 * time.Minute,
				HealthInterval: 1*time.Hour + 30*time.Minute,
				NoDefault:      0, // No default, remains zero
				RequiredDur:    0, // Required but no default, remains zero
			},
			wantErr: false,
		},
		{
			name: "existing values not overwritten",
			cfg: &DurationTestConfig{
				RequestTimeout: 60 * time.Second,
				CacheTTL:       10 * time.Minute,
			},
			expected: &DurationTestConfig{
				RequestTimeout: 60 * time.Second, // Not overwritten
				CacheTTL:       10 * time.Minute, // Not overwritten
				HealthInterval: 1*time.Hour + 30*time.Minute,
				NoDefault:      0,
				RequiredDur:    0,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ProcessConfigDefaults(tc.cfg)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected.RequestTimeout, tc.cfg.RequestTimeout)
			assert.Equal(t, tc.expected.CacheTTL, tc.cfg.CacheTTL)
			assert.Equal(t, tc.expected.HealthInterval, tc.cfg.HealthInterval)
			assert.Equal(t, tc.expected.NoDefault, tc.cfg.NoDefault)
			assert.Equal(t, tc.expected.RequiredDur, tc.cfg.RequiredDur)
		})
	}
}

func TestProcessConfigDefaults_TimeDuration_InvalidFormat(t *testing.T) {
	// Test config with invalid duration default
	type InvalidDurationConfig struct {
		Timeout time.Duration `default:"invalid_duration"`
	}

	cfg := &InvalidDurationConfig{}
	err := ProcessConfigDefaults(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse duration value")
}

func TestValidateConfig_TimeDuration_Integration(t *testing.T) {
	// Test complete validation flow with duration defaults
	cfg := &DurationTestConfig{
		RequiredDur: 15 * time.Second, // Set required field
	}

	err := ValidateConfig(cfg)
	require.NoError(t, err)

	// Verify defaults were applied
	assert.Equal(t, 30*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 5*time.Minute, cfg.CacheTTL)
	assert.Equal(t, 1*time.Hour+30*time.Minute, cfg.HealthInterval)
	assert.Equal(t, time.Duration(0), cfg.NoDefault)
	assert.Equal(t, 15*time.Second, cfg.RequiredDur)
}

func TestValidateConfig_TimeDuration_RequiredFieldMissing(t *testing.T) {
	// Test that required duration field validation works
	cfg := &DurationTestConfig{
		// RequiredDur not set
	}

	err := ValidateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RequiredDur")
}

func TestGenerateSampleConfig_TimeDuration(t *testing.T) {
	cfg := &DurationTestConfig{}

	// Test YAML generation
	yamlData, err := GenerateSampleConfig(cfg, "yaml")
	require.NoError(t, err)

	yamlStr := string(yamlData)
	assert.Contains(t, yamlStr, "request_timeout: 30s")
	assert.Contains(t, yamlStr, "cache_ttl: 5m0s")
	assert.Contains(t, yamlStr, "health_interval: 1h30m0s")

	// Test JSON generation
	jsonData, err := GenerateSampleConfig(cfg, "json")
	require.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, "30000000000")  // 30s in nanoseconds
	assert.Contains(t, jsonStr, "300000000000") // 5m in nanoseconds
}

func TestConfigFeederAndDefaults_TimeDuration_Integration(t *testing.T) {
	// Test that config feeders and defaults work together properly

	// Create test YAML file with some duration values
	yamlContent := `request_timeout: 45s
cache_ttl: 10m
# health_interval not set - should use default
required_dur: 2h`

	yamlFile := "/tmp/test_duration_integration.yaml"
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0600)
	require.NoError(t, err)
	defer os.Remove(yamlFile)

	cfg := &DurationTestConfig{}

	// First apply config feeder
	yamlFeeder := feeders.NewYamlFeeder(yamlFile)
	err = yamlFeeder.Feed(cfg)
	require.NoError(t, err)

	// Then apply defaults (this is what ValidateConfig does)
	err = ProcessConfigDefaults(cfg)
	require.NoError(t, err)

	// Verify that feeder values are preserved and defaults are applied where needed
	assert.Equal(t, 45*time.Second, cfg.RequestTimeout)             // From feeder
	assert.Equal(t, 10*time.Minute, cfg.CacheTTL)                   // From feeder
	assert.Equal(t, 1*time.Hour+30*time.Minute, cfg.HealthInterval) // Default (not in YAML)
	assert.Equal(t, 2*time.Hour, cfg.RequiredDur)                   // From feeder
	assert.Equal(t, time.Duration(0), cfg.NoDefault)                // No default, no feeder value
}

func TestEdgeCases_TimeDuration_Defaults(t *testing.T) {
	// Test edge cases for duration defaults

	t.Run("zero duration default", func(t *testing.T) {
		type ZeroDurationConfig struct {
			Timeout time.Duration `default:"0s"`
		}

		cfg := &ZeroDurationConfig{}
		err := ProcessConfigDefaults(cfg)
		require.NoError(t, err)
		assert.Equal(t, time.Duration(0), cfg.Timeout)
	})

	t.Run("very long duration default", func(t *testing.T) {
		type LongDurationConfig struct {
			Timeout time.Duration `default:"24h"`
		}

		cfg := &LongDurationConfig{}
		err := ProcessConfigDefaults(cfg)
		require.NoError(t, err)
		assert.Equal(t, 24*time.Hour, cfg.Timeout)
	})

	t.Run("complex duration default", func(t *testing.T) {
		type ComplexDurationConfig struct {
			Timeout time.Duration `default:"2h30m45s500ms"`
		}

		cfg := &ComplexDurationConfig{}
		err := ProcessConfigDefaults(cfg)
		require.NoError(t, err)
		expected := 2*time.Hour + 30*time.Minute + 45*time.Second + 500*time.Millisecond
		assert.Equal(t, expected, cfg.Timeout)
	})
}

func TestReverseProxyConfig_TimeDuration_Integration(t *testing.T) {
	// Test the actual reverseproxy module's HealthCheckConfig with duration defaults
	// This ensures our duration support works with the real-world config that was failing

	// Import reverseproxy config type
	type HealthCheckConfig struct {
		Enabled                bool          `json:"enabled" yaml:"enabled" toml:"enabled" env:"ENABLED" default:"false" desc:"Enable health checking for backend services"`
		Interval               time.Duration `json:"interval" yaml:"interval" toml:"interval" env:"INTERVAL" default:"30s" desc:"Interval between health checks"`
		Timeout                time.Duration `json:"timeout" yaml:"timeout" toml:"timeout" env:"TIMEOUT" default:"5s" desc:"Timeout for health check requests"`
		RecentRequestThreshold time.Duration `json:"recent_request_threshold" yaml:"recent_request_threshold" toml:"recent_request_threshold" env:"RECENT_REQUEST_THRESHOLD" default:"60s" desc:"Skip health check if a request to the backend occurred within this time"`
	}

	t.Run("defaults applied correctly", func(t *testing.T) {
		cfg := &HealthCheckConfig{}
		err := ProcessConfigDefaults(cfg)
		require.NoError(t, err)

		// Verify all duration defaults are applied correctly
		assert.False(t, cfg.Enabled)
		assert.Equal(t, 30*time.Second, cfg.Interval)
		assert.Equal(t, 5*time.Second, cfg.Timeout)
		assert.Equal(t, 60*time.Second, cfg.RecentRequestThreshold)
	})

	t.Run("config feeder overrides defaults", func(t *testing.T) {
		// Create test YAML file
		yamlContent := `enabled: true
interval: 45s
timeout: 10s
# recent_request_threshold not set - should use default`

		yamlFile := "/tmp/reverseproxy_health_test.yaml"
		err := os.WriteFile(yamlFile, []byte(yamlContent), 0600)
		require.NoError(t, err)
		defer os.Remove(yamlFile)

		cfg := &HealthCheckConfig{}

		// Apply config feeder first (normal flow)
		yamlFeeder := feeders.NewYamlFeeder(yamlFile)
		err = yamlFeeder.Feed(cfg)
		require.NoError(t, err)

		// Then apply defaults (this is what ValidateConfig does)
		err = ProcessConfigDefaults(cfg)
		require.NoError(t, err)

		// Verify feeder values preserved and defaults applied where needed
		assert.True(t, cfg.Enabled)                                 // From feeder
		assert.Equal(t, 45*time.Second, cfg.Interval)               // From feeder
		assert.Equal(t, 10*time.Second, cfg.Timeout)                // From feeder
		assert.Equal(t, 60*time.Second, cfg.RecentRequestThreshold) // Default (not in YAML)
	})

	t.Run("complete validation flow", func(t *testing.T) {
		cfg := &HealthCheckConfig{}

		// This is the complete flow that the application uses
		err := ValidateConfig(cfg)
		require.NoError(t, err)

		// Verify all defaults are applied
		assert.False(t, cfg.Enabled)
		assert.Equal(t, 30*time.Second, cfg.Interval)
		assert.Equal(t, 5*time.Second, cfg.Timeout)
		assert.Equal(t, 60*time.Second, cfg.RecentRequestThreshold)
	})
}
