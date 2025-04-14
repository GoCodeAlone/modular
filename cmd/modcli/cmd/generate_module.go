package cmd

import (
	"errors" // Added
	"fmt"
	"log/slog" // Added
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile" // Added
)

// SurveyStdio is a public variable to make it accessible for testing
var SurveyStdio = DefaultSurveyIO

// SetOptionsFn is used to override the survey prompts during testing
var SetOptionsFn func(*ModuleOptions) bool

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

// --- Template Definitions ---

// Define the main module template
// Use ` + "`" + ` within the string to represent backticks for struct tags
const moduleTmpl = `package {{.PackageName}}
// ... existing moduleTmpl content ...
` // End of moduleTmpl

// Define the module test template
const moduleTestTmpl = `package {{.PackageName}}

import (
	{{if or .HasStartupLogic .HasShutdownLogic}}"context"{{end}}
	"testing"
	{{if or .IsTenantAware .ProvidesServices .RequiresServices}}"github.com/GoCodeAlone/modular"{{end}}
	"github.com/stretchr/testify/assert"
	{{if or .HasConfig .IsTenantAware .ProvidesServices .RequiresServices}}"github.com/stretchr/testify/require"{{end}}
	{{if or .HasConfig .IsTenantAware}}"fmt"{{end}}
)

func TestNew{{.ModuleName}}Module(t *testing.T) {
	module := New{{.ModuleName}}Module()
	assert.NotNil(t, module)

	modImpl, ok := module.(*{{.ModuleName}}Module)
	require.True(t, ok)
	assert.Equal(t, "{{.PackageName}}", modImpl.Name())
	{{if .IsTenantAware}}assert.NotNil(t, modImpl.tenantConfigs){{end}}
}

{{if .HasConfig}}
func TestModule_RegisterConfig(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	mockApp := NewMockApplication()
	err := module.RegisterConfig(mockApp)
	assert.NoError(t, err)
	assert.NotNil(t, module.config)
	_, err = mockApp.GetConfigSection(module.Name())
	assert.NoError(t, err, "Config section should be registered")
}
{{end}}

func TestModule_Init(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	mockApp := NewMockApplication()
	{{if .RequiresServices}}
	{{end}}
	err := module.Init(mockApp)
	assert.NoError(t, err)
}

{{if .HasStartupLogic}}
func TestModule_Start(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	err := module.Start(context.Background())
	assert.NoError(t, err)
}
{{end}}

{{if .HasShutdownLogic}}
func TestModule_Stop(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	err := module.Stop(context.Background())
	assert.NoError(t, err)
}
{{end}}

{{if .IsTenantAware}}
func TestModule_TenantLifecycle(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	{{if .HasConfig}}
	module.config = &Config{}
	{{end}}

	tenantID := modular.TenantID("test-tenant")
	module.OnTenantRegistered(tenantID)

	{{if .HasConfig}}
	mockTenantService := &MockTenantService{
		Configs: map[modular.TenantID]map[string]modular.ConfigProvider{
			tenantID: {
				module.Name(): modular.NewStdConfigProvider(&Config{}),
			},
		},
	}
	err := module.LoadTenantConfig(mockTenantService, tenantID)
	assert.NoError(t, err)
	loadedConfig := module.GetTenantConfig(tenantID)
	require.NotNil(t, loadedConfig, "Loaded tenant config should not be nil")
	{{else}}
	{{end}}

	module.OnTenantRemoved(tenantID)
	_, exists := module.tenantConfigs[tenantID]
	assert.False(t, exists, "Tenant config should be removed")
}

type MockTenantService struct {
	Configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (m *MockTenantService) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantSections, ok := m.Configs[tid]; ok {
		if provider, ok := tenantSections[section]; ok {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("mock config not found for tenant %s, section %s", tid, section)
}
func (m *MockTenantService) GetTenants() []modular.TenantID { return nil }
func (m *MockTenantService) RegisterTenant(modular.TenantID, map[string]modular.ConfigProvider) error { return nil }
func (m *MockTenantService) RemoveTenant(modular.TenantID) error { return nil }
func (m *MockTenantService) RegisterTenantAwareModule(modular.TenantAwareModule) error { return nil }

{{end}}

` // End of moduleTestTmpl

// Define the mock application template separately
const mockAppTmpl = `package {{.PackageName}}
// ... existing mockAppTmpl content ...
` // End of mockAppTmpl

// --- End Template Definitions ---

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
			if err := GenerateModuleFiles(options); err != nil {
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
	// For testing: bypass prompts and directly set options
	if SetOptionsFn != nil && SetOptionsFn(options) {
		return nil
	}

	// If module name not provided via flag, prompt for it
	if options.ModuleName == "" {
		namePrompt := &survey.Input{
			Message: "What is the name of your module?",
			Help:    "This will be used as the unique identifier for your module.",
		}
		if err := survey.AskOne(namePrompt, &options.ModuleName, survey.WithValidator(survey.Required), SurveyStdio.WithStdio()); err != nil {
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
	}, &answers, SurveyStdio.WithStdio())

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

	if err := survey.AskOne(formatQuestion, &configOptions.TagTypes, SurveyStdio.WithStdio()); err != nil {
		return err
	}

	// Ask if sample config files should be generated
	generateSampleQuestion := &survey.Confirm{
		Message: "Generate sample configuration files?",
		Default: true,
	}

	if err := survey.AskOne(generateSampleQuestion, &configOptions.GenerateSample, SurveyStdio.WithStdio()); err != nil {
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
		if err := survey.AskOne(nameQuestion, &field.Name, survey.WithValidator(survey.Required), SurveyStdio.WithStdio()); err != nil {
			return err
		}

		// Ask for the field type
		typeQuestion := &survey.Select{
			Message: "Field type:",
			Options: []string{"string", "int", "bool", "float64", "[]string", "[]int", "map[string]string", "struct (nested)"},
			Default: "string",
		}

		var fieldType string
		if err := survey.AskOne(typeQuestion, &fieldType, SurveyStdio.WithStdio()); err != nil {
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
		if err := survey.AskOne(requiredQuestion, &field.IsRequired, SurveyStdio.WithStdio()); err != nil {
			return err
		}

		// Ask for a default value
		defaultQuestion := &survey.Input{
			Message: "Default value (leave empty for none):",
			Help:    "The default value for this field, if any",
		}
		if err := survey.AskOne(defaultQuestion, &field.DefaultValue, SurveyStdio.WithStdio()); err != nil {
			return err
		}

		// Ask for a description
		descQuestion := &survey.Input{
			Message: "Description:",
			Help:    "A brief description of what this field is used for",
		}
		if err := survey.AskOne(descQuestion, &field.Description, SurveyStdio.WithStdio()); err != nil {
			return err
		}

		// Add the field
		configOptions.Fields = append(configOptions.Fields, field)

		// Ask if more fields should be added
		addMoreQuestion := &survey.Confirm{
			Message: "Add another field?",
			Default: true,
		}
		if err := survey.AskOne(addMoreQuestion, &addFields, SurveyStdio.WithStdio()); err != nil {
			return err
		}
	}

	return nil
}

// GenerateModuleFiles generates all the files for the module
func GenerateModuleFiles(options *ModuleOptions) error {
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

	// Generate go.mod file
	if err := generateGoModFile(outputDir, options); err != nil {
		return fmt.Errorf("failed to generate go.mod file: %w", err)
	}

	return nil
}

// generateModuleFile creates the main module.go file
func generateModuleFile(outputDir string, options *ModuleOptions) error {
	moduleTmpl := `package {{.PackageName}}

import (
	{{if or .HasStartupLogic .HasShutdownLogic}}"context"{{end}} {{/* Conditionally import context */}}
	{{if or .HasConfig .IsTenantAware .ProvidesServices .RequiresServices}}"github.com/GoCodeAlone/modular"{{end}} {{/* Conditionally import modular */}}
	{{if .HasConfig}}"log/slog"{{end}} {{/* Conditionally import slog */}}
	{{if .HasConfig}}"fmt"{{end}} {{/* Conditionally import fmt */}}
)

{{if .HasConfig}}
// Config holds the configuration for the {{.ModuleName}} module
type Config struct {
	// Add configuration fields here
	// ExampleField string ` + "`mapstructure:\"example_field\"`" + `
}

// ProvideDefaults sets default values for the configuration
func (c *Config) ProvideDefaults() {
	// Set default values here
	// c.ExampleField = "default_value"
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Add validation logic here
	// if c.ExampleField == "" {
	//     return fmt.Errorf("example_field cannot be empty")
	// }
	return nil
}
{{end}}

// {{.ModuleName}}Module represents the {{.ModuleName}} module
type {{.ModuleName}}Module struct {
	name string
	{{if .HasConfig}}config *Config{{end}}
	{{if .IsTenantAware}}tenantConfigs map[modular.TenantID]*Config{{end}}
	// Add other dependencies or state fields here
}

// New{{.ModuleName}}Module creates a new instance of the {{.ModuleName}} module
func New{{.ModuleName}}Module() modular.Module {
	return &{{.ModuleName}}Module{
		name: "{{.PackageName}}",
		{{if .IsTenantAware}}tenantConfigs: make(map[modular.TenantID]*Config),{{end}}
	}
}

// Name returns the name of the module
func (m *{{.ModuleName}}Module) Name() string {
	return m.name
}

{{if .HasConfig}}
// RegisterConfig registers the module's configuration structure
func (m *{{.ModuleName}}Module) RegisterConfig(app modular.Application) error {
	m.config = &Config{} // Initialize with defaults or empty struct
	if err := app.RegisterConfigSection(m.Name(), m.config); err != nil { // Check error from RegisterConfigSection
		return fmt.Errorf("failed to register config section for module %s: %w", m.Name(), err)
	}
	// Load initial config values if needed (e.g., from app's main provider)
	// Note: Config values will be populated later by feeders during app.Init()
	slog.Debug("Registered config section", "module", m.Name())
	return nil
}
{{end}}

// Init initializes the module
func (m *{{.ModuleName}}Module) Init(app modular.Application) error {
	{{if .HasConfig}}slog.Info("Initializing {{.ModuleName}} module"){{else}}// Add initialization logging if desired{{end}}
	{{if .RequiresServices}}
	// Example: Resolve service dependencies
	// var myService MyServiceType
	// if err := app.GetService("myServiceName", &myService); err != nil {
	//     return fmt.Errorf("failed to get service 'myServiceName': %w", err)
	// }
	// m.myService = myService
	{{end}}
	// Add module initialization logic here
	return nil
}

{{if .HasStartupLogic}}
// Start performs startup logic for the module
func (m *{{.ModuleName}}Module) Start(ctx context.Context) error {
	{{if .HasConfig}}slog.Info("Starting {{.ModuleName}} module"){{else}}// Add startup logging if desired{{end}}
	// Add module startup logic here
	return nil
}
{{end}}

{{if .HasShutdownLogic}}
// Stop performs shutdown logic for the module
func (m *{{.ModuleName}}Module) Stop(ctx context.Context) error {
	{{if .HasConfig}}slog.Info("Stopping {{.ModuleName}} module"){{else}}// Add shutdown logging if desired{{end}}
	// Add module shutdown logic here
	return nil
}
{{end}}

{{if .HasDependencies}}
// Dependencies returns the names of modules this module depends on
func (m *{{.ModuleName}}Module) Dependencies() []string {
	// return []string{"otherModule"} // Add dependencies here
	return nil
}
{{end}}

{{if .ProvidesServices}}
// ProvidesServices declares services provided by this module
func (m *{{.ModuleName}}Module) ProvidesServices() []modular.ServiceProvider {
	// return []modular.ServiceProvider{
	//     {Name: "myService", Instance: myServiceImpl},
	// }
	return nil
}
{{end}}

{{if .RequiresServices}}
// RequiresServices declares services required by this module
func (m *{{.ModuleName}}Module) RequiresServices() []modular.ServiceDependency {
	// return []modular.ServiceDependency{
	//     {Name: "requiredService", Optional: false},
	// }
	return nil
}
{{end}}

{{if .IsTenantAware}}
// OnTenantRegistered is called when a new tenant is registered
func (m *{{.ModuleName}}Module) OnTenantRegistered(tenantID modular.TenantID) {
	{{if .HasConfig}}slog.Info("Tenant registered in {{.ModuleName}} module", "tenantID", tenantID){{else}}// Add tenant registration logging if desired{{end}}
	// Perform actions when a tenant is added, e.g., initialize tenant-specific resources
}

// OnTenantRemoved is called when a tenant is removed
func (m *{{.ModuleName}}Module) OnTenantRemoved(tenantID modular.TenantID) {
	{{if .HasConfig}}slog.Info("Tenant removed from {{.ModuleName}} module", "tenantID", tenantID){{else}}// Add tenant removal logging if desired{{end}}
	// Perform cleanup for the removed tenant
	delete(m.tenantConfigs, tenantID)
}

// LoadTenantConfig loads the configuration for a specific tenant
func (m *{{.ModuleName}}Module) LoadTenantConfig(tenantService modular.TenantService, tenantID modular.TenantID) error {
	configProvider, err := tenantService.GetTenantConfig(tenantID, m.Name())
	if err != nil {
		// Handle cases where config might be optional for a tenant
		{{if .HasConfig}}slog.Warn("No specific config found for tenant, using defaults/base.", "module", m.Name(), "tenantID", tenantID){{end}}
		// If config is required, return error:
		// return fmt.Errorf("failed to get config for tenant %s in module %s: %w", tenantID, m.Name(), err)
		{{if .HasConfig}}m.tenantConfigs[tenantID] = m.config{{end}} // Use base config as fallback
		return nil
	}

	tenantCfg := &Config{} // Create a new config struct for the tenant
	// It's crucial to clone or create a new instance to avoid tenants sharing config objects.
	// A simple approach is to unmarshal into a new struct.
	if err := configProvider.Unmarshal(tenantCfg); err != nil {
		return fmt.Errorf("failed to unmarshal config for tenant %s in module %s: %w", tenantID, m.Name(), err)
	}

	m.tenantConfigs[tenantID] = tenantCfg
	{{if .HasConfig}}slog.Debug("Loaded config for tenant", "module", m.Name(), "tenantID", tenantID){{end}}
	return nil
}

// GetTenantConfig retrieves the loaded configuration for a specific tenant
// Returns the base config if no specific tenant config is found.
func (m *{{.ModuleName}}Module) GetTenantConfig(tenantID modular.TenantID) *Config {
	if cfg, ok := m.tenantConfigs[tenantID]; ok {
		return cfg
	}
	// Fallback to base config if tenant-specific config doesn't exist
	{{if .HasConfig}}return m.config{{else}}return nil{{end}}
}
{{end}}
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
	{{- range $nfield := .NestedFields}}
	{{template "configField" $nfield}}
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
		"last": func(index int, collection interface{}) bool {
			switch v := collection.(type) {
			case []ConfigField:
				return index == len(v)-1
			default:
				return false
			}
		},
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
	// Create function map for templates with the last function
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"last": func(index int, collection interface{}) bool {
			switch v := collection.(type) {
			case []ConfigField:
				return index == len(v)-1
			case []*ConfigField:
				return index == len(v)-1
			default:
				return false
			}
		},
	}

	// Sample template for YAML
	yamlTmpl := `{{.PackageName}}:
{{- range $field := .ConfigOptions.Fields}}
  {{$field.Name | ToLower}}: {{if $field.IsNested}}
    {{- range $nfield := $field.NestedFields}}
    {{$nfield.Name | ToLower}}: {{if eq $nfield.Type "string"}}"example value"{{else}}42{{end}}
    {{- end}}
  {{- else if $field.IsArray}}
    {{- if eq $field.Type "[]string"}}
    - "example string"
    - "another string"
    {{- else if eq $field.Type "[]int"}}
    - 1
    - 2
    {{- else}}
    - "value1"
    - "value2"
    {{- end}}
  {{- else if $field.IsMap}}
    key1: "value1"
    key2: "value2"
  {{- else if $field.DefaultValue}}
    {{- if eq $field.Type "string"}}"{{$field.DefaultValue}}"{{else}}{{$field.DefaultValue}}{{end}}
  {{- else if eq $field.Type "string"}}"example value"
  {{- else if eq $field.Type "int"}}42
  {{- else if eq $field.Type "bool"}}false
  {{- else if eq $field.Type "float64"}}3.14
  {{- else}}null
  {{- end}}
{{- end}}
`

	// Sample template for JSON
	jsonTmpl := `{
  "{{.PackageName}}": {
{{- range $i, $field := .ConfigOptions.Fields}}
    "{{$field.Name | ToLower}}": {{if $field.IsNested}}{
      {{- range $j, $nfield := $field.NestedFields}}
      "{{$nfield.Name | ToLower}}": {{if eq $nfield.Type "string"}}"example value"{{else}}42{{end}}{{if not (last $j $field.NestedFields)}},{{end}}
      {{- end}}
    }{{else if $field.IsArray}}[
      {{- if eq $field.Type "[]string"}}
      "example string",
      "another string"
      {{- else if eq $field.Type "[]int"}}
      1,
      2
      {{- else}}
      "value1",
      "value2"
      {{- end}}
    ]{{else if $field.IsMap}}{
      "key1": "value1",
      "key2": "value2"
    }{{else if $field.DefaultValue}}
      {{- if eq $field.Type "string"}}"{{$field.DefaultValue}}"{{else}}{{$field.DefaultValue}}{{end}}
    {{else if eq $field.Type "string"}}"example value"
    {{else if eq $field.Type "int"}}42
    {{else if eq $field.Type "bool"}}false
    {{else if eq $field.Type "float64"}}3.14
    {{else}}null
    {{end}}{{if not (last $i $.ConfigOptions.Fields)}},{{end}}
  }
{{- end}}
}`

	// Sample template for TOML
	tomlTmpl := `[{{.PackageName}}]
{{- range $field := .ConfigOptions.Fields}}
{{- if $field.IsNested}}
[{{$.PackageName}}.{{$field.Name | ToLower}}]
{{- range $nfield := $field.NestedFields}}
{{$nfield.Name | ToLower}} = {{if eq $nfield.Type "string"}}"example value"{{else}}42{{end}}
{{- end}}
{{- else if $field.IsArray}}
{{$field.Name | ToLower}} = {{if eq $field.Type "[]string"}}["example string", "another string"]{{else if eq $field.Type "[]int"}}[1, 2]{{else}}["value1", "value2"]{{end}}
{{- else if $field.IsMap}}
[{{$.PackageName}}.{{$field.Name | ToLower}}]
key1 = "value1"
key2 = "value2"
{{- else if $field.DefaultValue}}
{{$field.Name | ToLower}} = {{if eq $field.Type "string"}}"{{$field.DefaultValue}}"{{else}}{{$field.DefaultValue}}{{end}}
{{- else if eq $field.Type "string"}}
{{$field.Name | ToLower}} = "example value"
{{- else if eq $field.Type "int"}}
{{$field.Name | ToLower}} = 42
{{- else if eq $field.Type "bool"}}
{{$field.Name | ToLower}} = false
{{- else if eq $field.Type "float64"}}
{{$field.Name | ToLower}} = 3.14
{{- else}}
{{$field.Name | ToLower}} = nil
{{- end}}
{{- end}}
`

	// Check which formats to generate
	for _, format := range options.ConfigOptions.TagTypes {
		switch format {
		case "yaml":
			// Create YAML sample
			file, err := os.Create(filepath.Join(outputDir, "config-sample.yaml"))
			if err != nil {
				return fmt.Errorf("failed to create YAML sample file: %w", err)
			}
			defer file.Close()

			tmpl, err := template.New("yamlSample").Funcs(funcMap).Parse(yamlTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse YAML template: %w", err)
			}

			err = tmpl.Execute(file, options)
			if err != nil {
				return fmt.Errorf("failed to execute YAML template: %w", err)
			}

		case "json":
			// Create JSON sample
			file, err := os.Create(filepath.Join(outputDir, "config-sample.json"))
			if err != nil {
				return fmt.Errorf("failed to create JSON sample file: %w", err)
			}
			defer file.Close()

			// Fixed: Added funcMap to the JSON template
			tmpl, err := template.New("jsonSample").Funcs(funcMap).Parse(jsonTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse JSON template: %w", err)
			}

			err = tmpl.Execute(file, options)
			if err != nil {
				return fmt.Errorf("failed to execute JSON template: %w", err)
			}

		case "toml":
			// Create TOML sample
			file, err := os.Create(filepath.Join(outputDir, "config-sample.toml"))
			if err != nil {
				return fmt.Errorf("failed to create TOML sample file: %w", err)
			}
			defer file.Close()

			// Fixed: Added funcMap to the TOML template
			tmpl, err := template.New("tomlSample").Funcs(funcMap).Parse(tomlTmpl)
			if err != nil {
				return fmt.Errorf("failed to parse TOML template: %w", err)
			}

			err = tmpl.Execute(file, options)
			if err != nil {
				return fmt.Errorf("failed to execute TOML template: %w", err)
			}
		}
	}

	return nil
}

// generateTestFiles creates test files for the module
func generateTestFiles(outputDir string, options *ModuleOptions) error {
	// Define the test template separately to avoid backtick-related syntax errors
	testTmpl := `package {{.PackageName}}

import (
	{{if or .HasStartupLogic .HasShutdownLogic}}"context"{{end}} {{/* Conditionally import context */}}
	"testing"
	{{if or .IsTenantAware .ProvidesServices .RequiresServices}}"github.com/GoCodeAlone/modular"{{end}} {{/* Conditionally import modular */}}
	"github.com/stretchr/testify/assert"
	{{if or .HasConfig .IsTenantAware .ProvidesServices .RequiresServices}}"github.com/stretchr/testify/require"{{end}} {{/* Conditionally import require */}}
)

func TestNew{{.ModuleName}}Module(t *testing.T) {
	module := New{{.ModuleName}}Module()
	assert.NotNil(t, module)
	// Test module properties
	modImpl, ok := module.(*{{.ModuleName}}Module)
	require.True(t, ok) // Use require here as the rest of the test depends on this
	assert.Equal(t, "{{.PackageName}}", modImpl.Name())
	{{if .IsTenantAware}}assert.NotNil(t, modImpl.tenantConfigs){{end}}
}

{{if .HasConfig}}
func TestModule_RegisterConfig(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	// Create a mock application
	mockApp := NewMockApplication()
	// Test RegisterConfig
	err := module.RegisterConfig(mockApp)
	assert.NoError(t, err)
	assert.NotNil(t, module.config) // Verify config struct was initialized
	// Verify the config section was registered in the mock app
	_, err = mockApp.GetConfigSection(module.Name())
	assert.NoError(t, err, "Config section should be registered")
}
{{end}}

func TestModule_Init(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	// Create a mock application
	mockApp := NewMockApplication()
	{{if .RequiresServices}}
	// Register mock services if needed for Init
	// mockService := &MockMyService{}
	// mockApp.RegisterService("requiredService", mockService)
	{{end}}
	// Test Init
	err := module.Init(mockApp)
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Init
}

{{if .HasStartupLogic}}
func TestModule_Start(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	// Add setup if needed, e.g., call Init
	// mockApp := NewMockApplication()
	// module.Init(mockApp)

	// Test Start
	err := module.Start(context.Background())
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Start
}
{{end}}

{{if .HasShutdownLogic}}
func TestModule_Stop(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	// Add setup if needed, e.g., call Init and Start
	// mockApp := NewMockApplication()
	// module.Init(mockApp)
	// module.Start(context.Background())

	// Test Stop
	err := module.Stop(context.Background())
	assert.NoError(t, err)
	// Add assertions here to check the state of the module after Stop
}
{{end}}

{{if .IsTenantAware}}
func TestModule_TenantLifecycle(t *testing.T) {
	module := New{{.ModuleName}}Module().(*{{.ModuleName}}Module)
	{{if .HasConfig}}
	// Initialize base config if needed for tenant fallback
	module.config = &Config{}
	{{end}}

	tenantID := modular.TenantID("test-tenant")
	// Test tenant registration
	module.OnTenantRegistered(tenantID)
	// Add assertions: check if tenant-specific resources were created
	// Test loading tenant config (requires a mock TenantService)
	mockTenantService := &MockTenantService{
		Configs: map[modular.TenantID]map[string]modular.ConfigProvider{
			tenantID: {
				module.Name(): modular.NewStdConfigProvider(&Config{ /* Populate with test data */ }),
			},
		},
	}
	err := module.LoadTenantConfig(mockTenantService, tenantID)
	assert.NoError(t, err)
	loadedConfig := module.GetTenantConfig(tenantID)
	require.NotNil(t, loadedConfig, "Loaded tenant config should not be nil")
	// Add assertions to check the loaded config values
	{{if .HasConfig}}
	// Test tenant removal
	module.OnTenantRemoved(tenantID)
	_, exists := module.tenantConfigs[tenantID]
	assert.False(t, exists, "Tenant config should be removed")
	// Add assertions: check if tenant-specific resources were cleaned up
}
	// Test tenant registration
// MockTenantService for testing LoadTenantConfig
type MockTenantService struct {
	Configs map[modular.TenantID]map[string]modular.ConfigProvider
}

func (m *MockTenantService) GetTenantConfig(tid modular.TenantID, section string) (modular.ConfigProvider, error) {
	if tenantSections, ok := m.Configs[tid]; ok {
		if provider, ok := tenantSections[section]; ok {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("mock config not found for tenant %s, section %s", tid, section)
}
func (m *MockTenantService) GetTenants() []modular.TenantID { return nil } // Not needed for this test
func (m *MockTenantService) RegisterTenant(modular.TenantID, map[string]modular.ConfigProvider) error { return nil } // Not needed
func (m *MockTenantService) RemoveTenant(modular.TenantID) error { return nil } // Not needed
func (m *MockTenantService) RegisterTenantAwareModule(modular.TenantAwareModule) error { return nil } // Not needed

{{end}}

// Add more tests for specific module functionality
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

// generateGoModFile creates a go.mod file for the new module
func generateGoModFile(outputDir string, options *ModuleOptions) error {
	// Skip go.mod generation and tidy if running in test mode where manual creation might occur
	if os.Getenv("TESTING") == "1" {
		slog.Debug("TESTING=1 set, skipping automatic go.mod generation and tidy.")
		return nil
	}

	goModPath := filepath.Join(outputDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		slog.Debug("go.mod file already exists, skipping generation.", "path", goModPath)
		return nil // File already exists
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check for existing go.mod: %w", err)
	}

	// --- Find and parse parent go.mod ---
	parentGoModPath, err := findParentGoMod()
	var parentReplaceDirectives []*modfile.Replace
	if err != nil {
		slog.Warn("Could not find parent go.mod, generated go.mod will not include parent replace directives.", "error", err)
	} else {
		slog.Debug("Found parent go.mod", "path", parentGoModPath)
		parentGoModBytes, err := os.ReadFile(parentGoModPath)
		if err != nil {
			slog.Warn("Could not read parent go.mod, generated go.mod will not include parent replace directives.", "path", parentGoModPath, "error", err)
		} else {
			parentModFile, err := modfile.Parse(parentGoModPath, parentGoModBytes, nil)
			if err != nil {
				slog.Warn("Could not parse parent go.mod, generated go.mod will not include parent replace directives.", "path", parentGoModPath, "error", err)
			} else {
				parentReplaceDirectives = parentModFile.Replace
				slog.Debug("Successfully parsed parent replace directives.", "count", len(parentReplaceDirectives))
			}
		}
	}
	// --- End find and parse parent go.mod ---

	// Use a simple template for go.mod content
	// Require modular v0.0.0 - rely on replace directives or user's go get/tidy
	// Require testify for generated tests
	// Construct a plausible module path based on the module name
	modulePath := fmt.Sprintf("example.com/%s", strings.ToLower(options.ModuleName))
	goModContent := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/GoCodeAlone/modular v0.0.0
	github.com/stretchr/testify v1.10.0
)`, modulePath) // Use the constructed module path

	// Append parent replace directives if found
	if len(parentReplaceDirectives) > 0 {
		goModContent += "\nreplace (\n"
		for _, rep := range parentReplaceDirectives {
			goModContent += fmt.Sprintf("\t%s => %s\n", rep.Old.Path, rep.New.Path)
			if rep.New.Version != "" {
				goModContent = strings.TrimSuffix(goModContent, "\n") + " " + rep.New.Version + "\n"
			}
		}
		goModContent += ")\n"
	}

	err = os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write go.mod file: %w", err)
	}
	slog.Debug("Successfully created go.mod file.", "path", goModPath)

	// Run 'go mod tidy'
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = outputDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("go mod tidy failed after generating go.mod. Manual check might be needed.", "output", string(output), "error", err)
		// Don't return error, as it might be due to environment issues not critical to generation
	} else {
		slog.Debug("Successfully ran go mod tidy.", "output", string(output))
	}

	return nil
}

// findParentGoMod searches upwards from the current directory for a go.mod file.
func findParentGoMod() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil // Found it
		} else if !errors.Is(err, os.ErrNotExist) {
			// Error other than not found
			return "", fmt.Errorf("error checking for go.mod at %s: %w", goModPath, err)
		}

		// Move up one directory
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			// Reached the root
			break
		}
		dir = parentDir
	}

	return "", errors.New("go.mod file not found in any parent directory")
}
