package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoCodeAlone/modular/cmd/modcli/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfigCommand(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := os.MkdirTemp("", "modular-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test data
	options := &cmd.ConfigOptions{
		Name:           "TestConfig",
		TagTypes:       []string{"yaml", "json"},
		GenerateSample: true,
		Fields: []cmd.ConfigField{
			{
				Name:        "ServerAddress",
				Type:        "string",
				Description: "The address the server listens on",
				IsRequired:  true,
				Tags:        []string{"yaml", "json"},
			},
			{
				Name:         "Port",
				Type:         "int",
				Description:  "The port the server listens on",
				DefaultValue: "8080",
				Tags:         []string{"yaml", "json"},
			},
			{
				Name:        "Debug",
				Type:        "bool",
				Description: "Enable debug mode",
				Tags:        []string{"yaml", "json"},
			},
		},
	}

	// Call the function to generate the config file
	err = cmd.GenerateStandaloneConfigFile(tmpDir, options)
	require.NoError(t, err)

	// Verify the config file was created
	configFilePath := filepath.Join(tmpDir, "testconfig.go")
	_, err = os.Stat(configFilePath)
	require.NoError(t, err, "Config file should exist")

	// Verify the file content
	content, err := os.ReadFile(configFilePath)
	require.NoError(t, err)

	// Check that the content includes the expected struct definition
	assert.Contains(t, string(content), "type TestConfig struct {")
	assert.Contains(t, string(content), "ServerAddress string `yaml:\"serveraddress\" json:\"serveraddress\" validate:\"required\"")
	assert.Contains(t, string(content), "Port int `yaml:\"port\" json:\"port\" default:\"8080\"")
	assert.Contains(t, string(content), "Debug bool `yaml:\"debug\" json:\"debug\"")

	// Verify Validate method
	assert.Contains(t, string(content), "func (c *TestConfig) Validate() error {")

	// Call the function to generate sample config files
	err = cmd.GenerateStandaloneSampleConfigs(tmpDir, options)
	require.NoError(t, err)

	// Verify the sample files were created
	yamlSamplePath := filepath.Join(tmpDir, "config-sample.yaml")
	_, err = os.Stat(yamlSamplePath)
	require.NoError(t, err, "YAML sample file should exist")

	// Verify JSON sample file was created
	jsonSamplePath := filepath.Join(tmpDir, "config-sample.json")
	_, err = os.Stat(jsonSamplePath)
	require.NoError(t, err, "JSON sample file should exist")

	// Check YAML sample content
	yamlContent, err := os.ReadFile(yamlSamplePath)
	require.NoError(t, err)
	assert.Contains(t, string(yamlContent), "serveraddress:")
	assert.Contains(t, string(yamlContent), "port:")
	assert.Contains(t, string(yamlContent), "debug:")

	// Check JSON sample content
	jsonContent, err := os.ReadFile(jsonSamplePath)
	require.NoError(t, err)
	assert.Contains(t, string(jsonContent), "\"serveraddress\":")
	assert.Contains(t, string(jsonContent), "\"port\":")
	assert.Contains(t, string(jsonContent), "\"debug\":")
}
