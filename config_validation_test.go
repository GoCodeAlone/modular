package modular

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

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
	Options     map[string]string `yaml:"options" default:"{\"key1\":\"value1\", \"key2\":\"value2\"}" desc:"Configuration options"`
	NestedCfg   *NestedTestConfig `yaml:"nested" desc:"Nested configuration"`
}

type NestedTestConfig struct {
	Enabled bool   `yaml:"enabled" default:"true" desc:"Enable the nested feature"`
	Timeout int    `yaml:"timeout" default:"30" desc:"Timeout in seconds"`
	ApiKey  string `yaml:"apiKey" required:"true" desc:"API key for authentication"`
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
					ApiKey: "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "missing environment",
			cfg: &ValidationTestConfig{
				Port: 8080,
				NestedCfg: &NestedTestConfig{
					ApiKey: "test-key",
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
			errorMsg: "ApiKey",
		},
		{
			name: "missing port",
			cfg: &ValidationTestConfig{
				Environment: "dev",
				NestedCfg: &NestedTestConfig{
					ApiKey: "test-key",
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
					ApiKey: "test-key",
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
					ApiKey: "test-key",
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
	defer os.Remove(tempFile)

	err := SaveSampleConfig(cfg, "yaml", tempFile)
	require.NoError(t, err)

	// Verify file exists and contains expected content
	fileData, err := os.ReadFile(tempFile)
	require.NoError(t, err)
	assert.Contains(t, string(fileData), "name: Default Name")
	assert.Contains(t, string(fileData), "port: 8080")
}
