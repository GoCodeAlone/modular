package cmd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular/cmd/modcli/cmd"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateConfigCommand(t *testing.T) {
	// Create a temporary directory for output
	tmpDir, err := os.MkdirTemp("", "modular-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test data
	options := &cmd.ConfigOptions{
		Name:           "TestConfig",
		TagTypes:       []string{"yaml", "json", "toml"}, // Added TOML for better coverage
		GenerateSample: true,
		Fields: []cmd.ConfigField{
			{
				Name:        "ServerAddress",
				Type:        "string",
				Description: "The address the server listens on",
				IsRequired:  true,
				Tags:        []string{"yaml", "json", "toml"},
			},
			{
				Name:         "Port",
				Type:         "int",
				Description:  "The port the server listens on",
				DefaultValue: "8080",
				Tags:         []string{"yaml", "json", "toml"},
			},
			{
				Name:        "Debug",
				Type:        "bool",
				Description: "Enable debug mode",
				Tags:        []string{"yaml", "json", "toml"},
			},
			{
				Name:        "Nested",
				Type:        "NestedConfig",
				IsNested:    true,
				Description: "Nested configuration",
				Tags:        []string{"yaml", "json", "toml"},
				NestedFields: []cmd.ConfigField{
					{Name: "Key", Type: "string", Tags: []string{"yaml", "json", "toml"}},
					{Name: "Value", Type: "int", Tags: []string{"yaml", "json", "toml"}},
				},
			},
		},
	}

	// Generate the config struct file
	// Create a 'config' subdirectory for the Go file
	configGoDir := filepath.Join(tmpDir, "config")
	err = os.MkdirAll(configGoDir, 0755)
	require.NoError(t, err)
	err = cmd.GenerateStandaloneConfigFile(configGoDir, options) // Generate into subdir
	require.NoError(t, err)

	// Generate sample configuration files in the root temp dir
	err = cmd.GenerateStandaloneSampleConfigs(tmpDir, options)
	require.NoError(t, err)

	// --- Verify generated Go file ---
	// The file should be named based on the config struct name + .go
	goFileName := strings.ToLower(options.Name) + ".go"
	goFilePath := filepath.Join(configGoDir, goFileName) // Look in subdir
	assert.FileExists(t, goFilePath)

	// --- Verify Go file compilation ---
	// Create a dummy main.go in the root temp dir
	mainContent := fmt.Sprintf(`
package main

import (
	_ "example.com/testmod/config" // Import the generated package
)

func main() {
	// We just need to ensure it builds
}
`) // Removed unused variable

	mainFilePath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainFilePath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create a dummy go.mod in the root temp dir
	goModContent := `
module example.com/testmod

go 1.21
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	err = os.WriteFile(goModPath, []byte(goModContent), 0644)
	require.NoError(t, err)

	// Run go build in the root temp dir
	buildCmd := exec.Command("go", "build", "-o", "/dev/null", ".") // Build in the temp dir
	buildCmd.Dir = tmpDir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	assert.NoError(t, buildErr, "Generated Go config file failed to compile: %s", string(buildOutput))

	// --- Verify sample files ---
	// Check YAML sample
	yamlSamplePath := filepath.Join(tmpDir, "config-sample.yaml")
	assert.FileExists(t, yamlSamplePath)
	yamlContent, err := os.ReadFile(yamlSamplePath)
	require.NoError(t, err)
	var yamlData interface{}
	err = yaml.Unmarshal(yamlContent, &yamlData)
	assert.NoError(t, err, "Failed to parse generated YAML sample")
	assert.NotEmpty(t, yamlData, "Parsed YAML data should not be empty")

	// Check JSON sample
	jsonSamplePath := filepath.Join(tmpDir, "config-sample.json")
	assert.FileExists(t, jsonSamplePath)
	jsonContent, err := os.ReadFile(jsonSamplePath)
	require.NoError(t, err)
	var jsonData interface{}
	// Use json.Unmarshal which handles trailing commas in Go 1.21+
	// If using older Go, the regex approach might be needed, but let's rely on stdlib
	err = json.Unmarshal(jsonContent, &jsonData)
	assert.NoError(t, err, "Failed to parse generated JSON sample")
	assert.NotEmpty(t, jsonData, "Parsed JSON data should not be empty")

	// Check TOML sample
	tomlSamplePath := filepath.Join(tmpDir, "config-sample.toml")
	assert.FileExists(t, tomlSamplePath)
	tomlContent, err := os.ReadFile(tomlSamplePath)
	require.NoError(t, err)
	var tomlData interface{}
	err = toml.Unmarshal(tomlContent, &tomlData)
	assert.NoError(t, err, "Failed to parse generated TOML sample")
	assert.NotEmpty(t, tomlData, "Parsed TOML data should not be empty")

}
