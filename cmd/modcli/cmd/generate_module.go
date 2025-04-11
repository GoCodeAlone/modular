package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// ModuleOptions contains the configuration for generating a new module
type ModuleOptions struct {
	ModuleName       string
	PackageName      string
	OutputDir        string
	HasConfig        bool
	IsTenantAware    bool
	HasDependencies  bool
	HasStartupLogic  bool
	HasShutdownLogic bool
	ProvidesServices bool
	RequiresServices bool
	GenerateTests    bool
	ConfigOptions    *ConfigOptions
}

// ConfigOptions contains the configuration for generating a module's config
type ConfigOptions struct {
	TagTypes       []string // yaml, json, toml, env
	GenerateSample bool
	Fields         []ConfigField
}

// ConfigField represents a field in the config struct
type ConfigField struct {
	Name         string
	Type         string
	IsRequired   bool
	DefaultValue string
	Description  string
	IsNested     bool
	NestedFields []ConfigField
	IsArray      bool
	IsMap        bool
	KeyType      string   // For maps
	ValueType    string   // For maps
	Tags         []string // For tracking which tags to include (yaml, json, toml, env)
}

// NewGenerateModuleCommand creates a command for generating Modular modules
func NewGenerateModuleCommand() *cobra.Command {
	var outputDir string
	var moduleName string

	cmd := &cobra.Command{
		Use:   "module",
		Short: "Generate a new Modular module",
		Long:  `Generate a new module for the Modular framework with the specified features.`,
		Run: func(cmd *cobra.Command, args []string) {
			options := &ModuleOptions{
				OutputDir:     outputDir,
				ModuleName:    moduleName,
				ConfigOptions: &ConfigOptions{},
			}

			// Collect module information through prompts
			if err := promptForModuleInfo(options); err != nil {
				fmt.Fprintf(os.Stderr, "Error gathering module information: %s\n", err)
				os.Exit(1)
			}

			// Generate the module files
			if err := generateModuleFiles(options); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating module: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully generated module '%s' in %s\n", options.ModuleName, options.OutputDir)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Directory where the module will be generated")
	cmd.Flags().StringVarP(&moduleName, "name", "n", "", "Name of the module to generate")

	return cmd
}

// promptForModuleInfo collects information about the module to generate
func promptForModuleInfo(options *ModuleOptions) error {
	// If module name not provided via flag, prompt for it
	if options.ModuleName == "" {
		namePrompt := &survey.Input{
			Message: "What is the name of your module?",
			Help:    "This will be used as the unique identifier for your module.",
		}
		if err := survey.AskOne(namePrompt, &options.ModuleName, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	// Determine package name (convert module name to lowercase and remove spaces)
	options.PackageName = strings.ToLower(strings.ReplaceAll(options.ModuleName, " ", ""))

	// Ask about module features
	featureQuestions := []*survey.Confirm{
		{
			Message: "Will this module have configuration?",
			Help:    "If yes, a config struct will be generated for this module.",
		},
		{
			Message: "Should this module be tenant-aware?",
			Help:    "If yes, the module will implement the TenantAwareModule interface.",
		},
		{
			Message: "Will this module depend on other modules?",
			Help:    "If yes, the module will implement the DependencyAware interface.",
		},
		{
			Message: "Does this module need to perform logic on startup (separate from init)?",
			Help:    "If yes, the module will implement the Startable interface.",
		},
		{
			Message: "Does this module need cleanup logic on shutdown?",
			Help:    "If yes, the module will implement the Stoppable interface.",
		},
		{
			Message: "Will this module provide services to other modules?",
			Help:    "If yes, the ProvidesServices method will be implemented.",
		},
		{
			Message: "Will this module require services from other modules?",
			Help:    "If yes, the RequiresServices method will be implemented.",
		},
		{
			Message: "Do you want to generate tests for this module?",
			Help:    "If yes, test files will be generated for the module.",
			Default: true,
		},
	}

	// Use a struct to hold our answers instead of an array
	type moduleFeatures struct {
		HasConfig        bool
		IsTenantAware    bool
		HasDependencies  bool
		HasStartupLogic  bool
		HasShutdownLogic bool
		ProvidesServices bool
		RequiresServices bool
		GenerateTests    bool
	}

	// Initialize with defaults
	answers := moduleFeatures{
		GenerateTests: true, // Default to true for test generation
	}

	err := survey.Ask([]*survey.Question{
		{
			Name:   "HasConfig",
			Prompt: featureQuestions[0],
		},
		{
			Name:   "IsTenantAware",
			Prompt: featureQuestions[1],
		},
		{
			Name:   "HasDependencies",
			Prompt: featureQuestions[2],
		},
		{
			Name:   "HasStartupLogic",
			Prompt: featureQuestions[3],
		},
		{
			Name:   "HasShutdownLogic",
			Prompt: featureQuestions[4],
		},
		{
			Name:   "ProvidesServices",
			Prompt: featureQuestions[5],
		},
		{
			Name:   "RequiresServices",
			Prompt: featureQuestions[6],
		},
		{
			Name:   "GenerateTests",
			Prompt: featureQuestions[7],
		},
	}, &answers)

	if err != nil {
		return err
	}

	// Copy the answers to our options struct
	options.HasConfig = answers.HasConfig
	options.IsTenantAware = answers.IsTenantAware
	options.HasDependencies = answers.HasDependencies
	options.HasStartupLogic = answers.HasStartupLogic
	options.HasShutdownLogic = answers.HasShutdownLogic
	options.ProvidesServices = answers.ProvidesServices
	options.RequiresServices = answers.RequiresServices
	options.GenerateTests = answers.GenerateTests

	// If module has configuration, collect config details
	if options.HasConfig {
		if err := promptForModuleConfigInfo(options.ConfigOptions); err != nil {
			return err
		}
	}

	return nil
}

// promptForModuleConfigInfo collects configuration field details for a module
func promptForModuleConfigInfo(configOptions *ConfigOptions) error {
	// Ask about the config format (YAML, JSON, TOML, etc.)
	formatQuestion := &survey.MultiSelect{
		Message: "Which config formats should be supported?",
		Options: []string{"yaml", "json", "toml", "env"},
		Default: []string{"yaml"},
	}

	if err := survey.AskOne(formatQuestion, &configOptions.TagTypes); err != nil {
		return err
	}

	// Ask if sample config files should be generated
	generateSampleQuestion := &survey.Confirm{
		Message: "Generate sample configuration files?",
		Default: true,
	}

	if err := survey.AskOne(generateSampleQuestion, &configOptions.GenerateSample); err != nil {
		return err
	}

	// Collect configuration fields
	configOptions.Fields = []ConfigField{}
	addFields := true

	for addFields {
		field := ConfigField{}

		// Ask for the field name
		nameQuestion := &survey.Input{
			Message: "Field name (CamelCase):",
			Help:    "The name of the configuration field (e.g., ServerAddress)",
		}
		if err := survey.AskOne(nameQuestion, &field.Name, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		// Ask for the field type
		typeQuestion := &survey.Select{
			Message: "Field type:",
			Options: []string{"string", "int", "bool", "float64", "[]string", "[]int", "map[string]string", "struct (nested)"},
			Default: "string",
		}

		var fieldType string
		if err := survey.AskOne(typeQuestion, &fieldType); err != nil {
			return err
		}

		// Set field type and special flags based on selection
		switch fieldType {
		case "struct (nested)":
			field.IsNested = true
			field.Type = field.Name + "Config" // Create a type name based on the field name
			// TODO: Add prompts for nested fields
		case "[]string", "[]int":
			field.IsArray = true
			field.Type = fieldType
		case "map[string]string":
			field.IsMap = true
			field.Type = fieldType
			field.KeyType = "string"
			field.ValueType = "string"
		default:
			field.Type = fieldType
		}

		// Ask if this field is required
		requiredQuestion := &survey.Confirm{
			Message: "Is this field required?",
			Default: false,
		}
		if err := survey.AskOne(requiredQuestion, &field.IsRequired); err != nil {
			return err
		}

		// Ask for a default value
		defaultQuestion := &survey.Input{
			Message: "Default value (leave empty for none):",
			Help:    "The default value for this field, if any",
		}
		if err := survey.AskOne(defaultQuestion, &field.DefaultValue); err != nil {
			return err
		}

		// Ask for a description
		descQuestion := &survey.Input{
			Message: "Description:",
			Help:    "A brief description of what this field is used for",
		}
		if err := survey.AskOne(descQuestion, &field.Description); err != nil {
			return err
		}

		// Add the field
		configOptions.Fields = append(configOptions.Fields, field)

		// Ask if more fields should be added
		addMoreQuestion := &survey.Confirm{
			Message: "Add another field?",
			Default: true,
		}
		if err := survey.AskOne(addMoreQuestion, &addFields); err != nil {
			return err
		}
	}

	return nil
}

// generateModuleFiles generates all the files for the module
func generateModuleFiles(options *ModuleOptions) error {
	// Create output directory if it doesn't exist
	outputDir := filepath.Join(options.OutputDir, options.PackageName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate module.go file
	if err := generateModuleFile(outputDir, options); err != nil {
		return fmt.Errorf("failed to generate module file: %w", err)
	}

	// Generate config.go if needed
	if options.HasConfig {
		if err := generateConfigFile(outputDir, options); err != nil {
			return fmt.Errorf("failed to generate config file: %w", err)
		}

		// Generate sample config files if requested
		if options.ConfigOptions.GenerateSample {
			if err := generateSampleConfigFiles(outputDir, options); err != nil {
				return fmt.Errorf("failed to generate sample config files: %w", err)
			}
		}
	}

	// Generate test files if requested
	if options.GenerateTests {
		if err := generateTestFiles(outputDir, options); err != nil {
			return fmt.Errorf("failed to generate test files: %w", err)
		}
	}

	// Generate README.md
	if err := generateReadmeFile(outputDir, options); err != nil {
		return fmt.Errorf("failed to generate README file: %w", err)
	}

	return nil
}

// generateModuleFile creates the main module.go file
func generateModuleFile(outputDir string, options *ModuleOptions) error {
	moduleTmpl := `package {{.PackageName}}

import (
	"context"
	"github.com/GoCodeAlone/modular"
)

// {{.ModuleName}}Module implements the Modular module interface
type {{.ModuleName}}Module struct {
	{{- if .HasConfig}}
	config *{{.ModuleName}}Config
	{{- end}}
	{{- if .IsTenantAware}}
	tenantConfigs map[modular.TenantID]*{{.ModuleName}}Config
	{{- end}}
}

// New{{.ModuleName}}Module creates a new instance of the {{.ModuleName}} module
func New{{.ModuleName}}Module() modular.Module {
	return &{{.ModuleName}}Module{
		{{- if .IsTenantAware}}
		tenantConfigs: make(map[modular.TenantID]*{{.ModuleName}}Config),
		{{- end}}
	}
}

// Name returns the unique identifier for this module
func (m *{{.ModuleName}}Module) Name() string {
	return "{{.PackageName}}"
}

{{- if .HasConfig}}
// RegisterConfig registers configuration requirements
func (m *{{.ModuleName}}Module) RegisterConfig(app modular.Application) error {
	m.config = &{{.ModuleName}}Config{
		// Default values can be set here
	}
	
	app.RegisterConfigSection("{{.PackageName}}", modular.NewStdConfigProvider(m.config))
	return nil
}
{{- end}}

// Init initializes the module
func (m *{{.ModuleName}}Module) Init(app modular.Application) error {
	// Initialize module resources
	
	return nil
}

{{- if .HasDependencies}}
// Dependencies returns names of other modules this module depends on
func (m *{{.ModuleName}}Module) Dependencies() []string {
	return []string{
		// Add dependencies here
	}
}
{{- end}}

{{- if or .ProvidesServices .RequiresServices}}
{{- if .ProvidesServices}}
// ProvidesServices returns a list of services provided by this module
func (m *{{.ModuleName}}Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		// Example:
		// {
		//     Name:        "serviceName",
		//     Description: "Description of the service",
		//     Instance:    serviceInstance,
		// },
	}
}
{{- end}}

{{- if .RequiresServices}}
// RequiresServices returns a list of services required by this module
func (m *{{.ModuleName}}Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		// Example:
		// {
		//     Name:     "requiredService",
		//     Required: true, // Whether this service is optional or required
		// },
	}
}
{{- end}}
{{- end}}

{{- if .HasStartupLogic}}
// Start is called when the application is starting
func (m *{{.ModuleName}}Module) Start(ctx context.Context) error {
	// Startup logic goes here
	
	return nil
}
{{- end}}

{{- if .HasShutdownLogic}}
// Stop is called when the application is shutting down
func (m *{{.ModuleName}}Module) Stop(ctx context.Context) error {
	// Shutdown/cleanup logic goes here
	
	return nil
}
{{- end}}

{{- if .IsTenantAware}}
// OnTenantRegistered is called when a new tenant is registered
func (m *{{.ModuleName}}Module) OnTenantRegistered(tenantID modular.TenantID) {
	// Initialize tenant-specific resources
}

// OnTenantRemoved is called when a tenant is removed
func (m *{{.ModuleName}}Module) OnTenantRemoved(tenantID modular.TenantID) {
	// Clean up tenant-specific resources
	delete(m.tenantConfigs, tenantID)
}
{{- end}}
`

	// Create and execute template
	tmpl, err := template.New("module").Parse(moduleTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse module template: %w", err)
	}

	// Create output file
	outputFile := filepath.Join(outputDir, "module.go")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create module file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, options); err != nil {
		return fmt.Errorf("failed to execute module template: %w", err)
	}

	return nil
}

// generateConfigFile creates the config.go file for a module
func generateConfigFile(outputDir string, options *ModuleOptions) error {
	// Create template definitions
	configTmpl := `package {{.PackageName}}

// {{.ModuleName}}Config holds the configuration for the {{.ModuleName}} module
type {{.ModuleName}}Config struct {
	{{- range .ConfigOptions.Fields}}
	{{template "configField" .}}
	{{- end}}
}

{{- range .ConfigOptions.Fields}}
{{- if .IsNested}}
// {{.Type}} holds nested configuration for {{.Name}}
type {{.Type}} struct {
	{{- range .NestedFields}}
	{{template "configField" .}}
	{{- end}}
}
{{- end}}
{{- end}}

// Validate implements the modular.ConfigValidator interface
func (c *{{.ModuleName}}Config) Validate() error {
	// Add custom validation logic here
	return nil
}
`

	fieldTmpl := `{{define "configField"}}{{.Name}} {{.Type}}{{if or .IsRequired .DefaultValue (len .Tags)}} ` + "`" + `{{range $i, $tag := $.Tags}}{{if $i}} {{end}}{{$tag}}:"{{$.Name | ToLower}}"{{end}}{{if .IsRequired}} required:"true"{{end}}{{if .DefaultValue}} default:"{{.DefaultValue}}"{{end}}{{if .Description}} desc:"{{.Description}}"{{end}}` + "`" + `{{end}}{{if .Description}} // {{.Description}}{{end}}{{end}}`

	// Create function map for templates
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// Create and execute template
	tmpl, err := template.New("config").Funcs(funcMap).Parse(configTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	// Add the field template
	_, err = tmpl.Parse(fieldTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse field template: %w", err)
	}

	// Create output file
	outputFile := filepath.Join(outputDir, "config.go")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Set tag information for fields
	for i := range options.ConfigOptions.Fields {
		options.ConfigOptions.Fields[i].Tags = options.ConfigOptions.TagTypes
	}

	// Execute template
	if err := tmpl.Execute(file, options); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	return nil
}

// generateSampleConfigFiles creates sample config files in the requested formats
func generateSampleConfigFiles(outputDir string, options *ModuleOptions) error {
	// Sample config template for YAML
	yamlTmpl := `# {{.ModuleName}} Module Configuration
{{- range .ConfigOptions.Fields}}
{{- if .Description}}
# {{.Description}}
{{- end}}
{{.Name | ToLower}}: {{template "yamlValue" .}}
{{- end}}`

	// Define the value template separately
	yamlValueTmpl := `{{define "yamlValue"}}{{if .IsNested}}
  {{- range .NestedFields}}
  {{.Name | ToLower}}: {{template "yamlValue" .}}
  {{- end}}
{{- else if .IsArray}}
  {{- if eq .Type "[]string"}}
  - "example string"
  - "another string"
  {{- else if eq .Type "[]int"}}
  - 1
  - 2
  {{- else if eq .Type "[]bool"}}
  - true
  - false
  {{- end}}
{{- else if .IsMap}}
  key1: value1
  key2: value2
{{- else if .DefaultValue}}
{{.DefaultValue}}
{{- else if eq .Type "string"}}
"example value"
{{- else if eq .Type "int"}}
42
{{- else if eq .Type "bool"}}
false
{{- else if eq .Type "float64"}}
3.14
{{- else}}
# TODO: Set appropriate value for {{.Type}}
{{- end}}{{end}}`

	// Create function map for templates
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// Check which formats to generate
	for _, format := range options.ConfigOptions.TagTypes {
		switch format {
		case "yaml":
			// Create YAML sample - create a new template each time
			tmpl := template.New("yamlSample").Funcs(funcMap)

			// First parse the value template, then the main template
			_, err := tmpl.Parse(yamlValueTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse YAML value template: %w", err)
			}

			_, err = tmpl.Parse(yamlTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse YAML template: %w", err)
			}

			outputFile := filepath.Join(outputDir, "config-sample.yaml")
			file, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create YAML sample file: %w", err)
			}

			if err := tmpl.ExecuteTemplate(file, "yamlSample", options); err != nil {
				file.Close()
				return fmt.Errorf("failed to execute YAML template: %w", err)
			}
			file.Close()

		case "toml", "json":
			// Similar implementation for TOML and JSON would go here
			// For brevity, I'm omitting these formats, but would follow a similar pattern
		}
	}

	return nil
}

// generateTestFiles creates test files for the module
func generateTestFiles(outputDir string, options *ModuleOptions) error {
	// Define the test template separately to avoid backtick-related syntax errors
	testTmpl := `package {{.PackageName}}

import (
	"context"
	"testing"
	
	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew{{.ModuleName}}Module(t *testing.T) {
	module := New{{.ModuleName}}Module()
	assert.NotNil(t, module)
	
	// Test module properties
	modImpl, ok := module.(*{{.ModuleName}}Module)
	require.True(t, ok)
	assert.Equal(t, "{{.PackageName}}", modImpl.Name())
	{{- if .IsTenantAware}}
	assert.NotNil(t, modImpl.tenantConfigs)
	{{- end}}
}

{{- if .HasConfig}}
func TestModule_RegisterConfig(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	
	// Create a mock application
	mockApp := &modular.MockApplication{}
	
	// Test RegisterConfig
	err := module.RegisterConfig(mockApp)
	assert.NoError(t, err)
	assert.NotNil(t, module.config)
}
{{- end}}

func TestModule_Init(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	
	// Create a mock application
	mockApp := &modular.MockApplication{}
	
	// Test Init
	err := module.Init(mockApp)
	assert.NoError(t, err)
}

{{- if .HasStartupLogic}}
func TestModule_Start(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	
	// Test Start
	err := module.Start(context.Background())
	assert.NoError(t, err)
}
{{- end}}

{{- if .HasShutdownLogic}}
func TestModule_Stop(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	
	// Test Stop
	err := module.Stop(context.Background())
	assert.NoError(t, err)
}
{{- end}}

{{- if .IsTenantAware}}
func TestModule_TenantLifecycle(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	
	// Test tenant registration
	tenantID := modular.TenantID("test-tenant")
	module.OnTenantRegistered(tenantID)
	
	// Test tenant removal
	module.OnTenantRemoved(tenantID)
	_, exists := module.tenantConfigs[tenantID]
	assert.False(t, exists)
}
{{- end}}
`

	// Define the mock application template separately
	mockAppTmpl := `package {{.PackageName}}

import (
	"github.com/GoCodeAlone/modular"
)

// MockApplication is a mock implementation of the modular.Application interface for testing
type MockApplication struct {
	ConfigSections map[string]modular.ConfigProvider
}

func NewMockApplication() *MockApplication {
	return &MockApplication{
		ConfigSections: make(map[string]modular.ConfigProvider),
	}
}

func (m *MockApplication) RegisterModule(module modular.Module) {
	// No-op for tests
}

func (m *MockApplication) RegisterService(name string, service interface{}) error {
	return nil
}

func (m *MockApplication) GetService(name string, target interface{}) error {
	return nil
}

func (m *MockApplication) RegisterConfigSection(name string, provider modular.ConfigProvider) {
	m.ConfigSections[name] = provider
}

func (m *MockApplication) Logger() modular.Logger {
	return nil
}
`

	// Create and execute test template
	tmpl, err := template.New("test").Parse(testTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse test template: %w", err)
	}

	// Create output file
	outputFile := filepath.Join(outputDir, "module_test.go")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, options); err != nil {
		return fmt.Errorf("failed to execute test template: %w", err)
	}

	// Create mock application if needed
	mockFile := filepath.Join(outputDir, "mock_test.go")
	mockFileExists := false
	if _, err := os.Stat(mockFile); err == nil {
		mockFileExists = true
	}

	if !mockFileExists {
		mockTmpl, err := template.New("mock").Parse(mockAppTmpl)
		if err != nil {
			return fmt.Errorf("failed to parse mock template: %w", err)
		}

		file, err := os.Create(mockFile)
		if err != nil {
			return fmt.Errorf("failed to create mock file: %w", err)
		}
		defer file.Close()

		if err := mockTmpl.Execute(file, options); err != nil {
			return fmt.Errorf("failed to execute mock template: %w", err)
		}
	}

	return nil
}

// generateReadmeFile creates a README.md file for the module
func generateReadmeFile(outputDir string, options *ModuleOptions) error {
	// Define the template as a raw string to avoid backtick-related syntax issues
	readmeContent := `# {{.ModuleName}} Module

A module for the [Modular](https://github.com/GoCodeAlone/modular) framework.

## Overview

The {{.ModuleName}} module provides... (describe your module here)

## Features

* Feature 1
* Feature 2
* Feature 3

## Installation

` + "```go" + `
go get github.com/yourusername/{{.PackageName}}
` + "```" + `

## Usage

` + "```go" + `
package main

import (
	"github.com/GoCodeAlone/modular"
	"github.com/yourusername/{{.PackageName}}"
	"log/slog"
	"os"
)

func main() {
	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
	)

	// Register the {{.ModuleName}} module
	app.RegisterModule({{.PackageName}}.New{{.ModuleName}}Module())

	// Run the application
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}
` + "```" + `

{{- if .HasConfig}}
## Configuration

The {{.ModuleName}} module supports the following configuration options:

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
{{- range .ConfigOptions.Fields}}
| {{.Name}} | {{.Type}} | {{if .IsRequired}}Yes{{else}}No{{end}} | {{if .DefaultValue}}{{.DefaultValue}}{{else}}-{{end}} | {{.Description}} |
{{- end}}

### Example Configuration

` + "```yaml" + `
# config.yaml
{{.PackageName}}:
{{- range .ConfigOptions.Fields}}
  {{.Name | ToLower}}: {{if .DefaultValue}}{{.DefaultValue}}{{else}}# Your value here{{end}}
{{- end}}
` + "```" + `
{{- end}}

## License

[MIT License](LICENSE)
`

	// Create function map for templates
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// Create and execute template
	tmpl, err := template.New("readme").Funcs(funcMap).Parse(readmeContent)
	if err != nil {
		return fmt.Errorf("failed to parse README template: %w", err)
	}

	// Create output file
	outputFile := filepath.Join(outputDir, "README.md")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create README file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, options); err != nil {
		return fmt.Errorf("failed to execute README template: %w", err)
	}

	return nil
}
