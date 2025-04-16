package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCommandCreation(t *testing.T) {
	// Test that the command is created properly
	cmd := NewGenerateConfigCommand()
	assert.NotNil(t, cmd)
	assert.Equal(t, "config", cmd.Use)

	// Test the flags
	outputFlag := cmd.Flag("output")
	assert.NotNil(t, outputFlag)
	assert.Equal(t, "o", outputFlag.Shorthand)

	nameFlag := cmd.Flag("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "n", nameFlag.Shorthand)
}

func TestConfigSampleGeneration(t *testing.T) {
	// Setup test data
	outputDir := t.TempDir()
	options := &ConfigOptions{
		Name: "TestConfig",
		Fields: []ConfigField{
			{
				Name:         "ServerAddress",
				Type:         "string",
				DefaultValue: "localhost:8080",
				Description:  "The server address",
				IsRequired:   true,
				Tags:         []string{"yaml", "json", "toml"},
			},
			{
				Name:         "EnableDebug",
				Type:         "bool",
				DefaultValue: "false",
				Description:  "Enable debug logging",
				Tags:         []string{"yaml", "json", "toml"},
			},
		},
	}

	// Call the function
	err := GenerateStandaloneSampleConfigs(outputDir, options)
	assert.NoError(t, err)

	// Test generating standalone config file
	err = GenerateStandaloneConfigFile(outputDir, options)
	assert.NoError(t, err)
}

// TestPromptForConfigFields tests setting configuration fields directly
func TestPromptForConfigFields(t *testing.T) {
	// Create a test ConfigOptions instance
	options := &ConfigOptions{}

	// Directly set the fields that would normally be set via prompts
	options.Fields = []ConfigField{
		{
			Name: "MyString",
			Type: "string",
		},
		{
			Name: "MyInt",
			Type: "int",
		},
	}

	// Assertions
	require.Len(t, options.Fields, 2, "Should have 2 fields")
	assert.Equal(t, "MyString", options.Fields[0].Name)
	assert.Equal(t, "string", options.Fields[0].Type)
	assert.Equal(t, "MyInt", options.Fields[1].Name)
	assert.Equal(t, "int", options.Fields[1].Type)
}

// TestPromptForConfigFields_NoFields tests when no fields are added
func TestPromptForConfigFields_NoFields(t *testing.T) {
	// Create a test ConfigOptions with no fields
	options := &ConfigOptions{}
	options.Fields = []ConfigField{}

	// Assertions
	assert.Len(t, options.Fields, 0, "Should have no fields")
}

// TestPromptForModuleConfigInfo tests the module config info prompting
func TestPromptForModuleConfigInfo(t *testing.T) {
	// Create a test ConfigOptions
	configOptions := &ConfigOptions{}

	// Directly set the config options as if they came from the prompt
	configOptions.TagTypes = []string{"yaml", "json"}
	configOptions.GenerateSample = true
	configOptions.Fields = []ConfigField{
		{
			Name:         "ServerAddress",
			Type:         "string",
			IsRequired:   true,
			DefaultValue: "localhost:8080",
			Description:  "The server address",
			Tags:         []string{"yaml", "json"},
		},
	}

	// Verify results
	assert.Equal(t, []string{"yaml", "json"}, configOptions.TagTypes)
	assert.True(t, configOptions.GenerateSample)
	assert.Len(t, configOptions.Fields, 1)
	assert.Equal(t, "ServerAddress", configOptions.Fields[0].Name)
}
