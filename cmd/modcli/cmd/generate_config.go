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

// NewGenerateConfigCommand creates a command for generating standalone config files
func NewGenerateConfigCommand() *cobra.Command {
	var outputDir string
	var configName string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate a new configuration file",
		Long:  `Generate a new configuration struct and optionally sample config files.`,
		Run: func(cmd *cobra.Command, args []string) {
			options := &ConfigOptions{
				Fields: []ConfigField{},
			}

			// Prompt for config name if not provided
			if configName == "" {
				namePrompt := &survey.Input{
					Message: "What is the name of your configuration?",
					Help:    "This will be used as the struct name for your config.",
				}
				if err := survey.AskOne(namePrompt, &configName, survey.WithValidator(survey.Required)); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
			}

			// Collect config details
			if err := promptForConfigInfo(options); err != nil {
				fmt.Fprintf(os.Stderr, "Error gathering config information: %s\n", err)
				os.Exit(1)
			}

			// Generate the config file
			if err := generateStandaloneConfigFile(outputDir, configName, options); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating config: %s\n", err)
				os.Exit(1)
			}

			// Generate sample config files if requested
			if options.GenerateSample {
				if err := generateStandaloneSampleConfigs(outputDir, configName, options); err != nil {
					fmt.Fprintf(os.Stderr, "Error generating sample configs: %s\n", err)
					os.Exit(1)
				}
			}

			fmt.Printf("Successfully generated config '%s' in %s\n", configName, outputDir)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Directory where the config will be generated")
	cmd.Flags().StringVarP(&configName, "name", "n", "", "Name of the config to generate")

	return cmd
}

// promptForConfigInfo collects information about the config structure
func promptForConfigInfo(options *ConfigOptions) error {
	// Prompt for tag types
	tagOptions := []string{"yaml", "json", "toml", "env"}
	tagPrompt := &survey.MultiSelect{
		Message: "Select tag types to include:",
		Options: tagOptions,
		Default: []string{"yaml", "json"},
	}
	if err := survey.AskOne(tagPrompt, &options.TagTypes); err != nil {
		return err
	}

	// Prompt for whether to generate sample config files
	generateSamplePrompt := &survey.Confirm{
		Message: "Generate sample config files?",
		Default: true,
	}
	if err := survey.AskOne(generateSamplePrompt, &options.GenerateSample); err != nil {
		return err
	}

	// Prompt for config fields
	if err := promptForConfigFields(&options.Fields); err != nil {
		return err
	}

	return nil
}

// promptForConfigFields collects information about config fields
func promptForConfigFields(fields *[]ConfigField) error {
	for {
		// Ask if user wants to add another field
		addMore := true
		if len(*fields) > 0 {
			addMorePrompt := &survey.Confirm{
				Message: "Add another field?",
				Default: true,
			}
			if err := survey.AskOne(addMorePrompt, &addMore); err != nil {
				return err
			}
		}

		if !addMore {
			break
		}

		// Collect field information
		field := ConfigField{}

		// Field name
		namePrompt := &survey.Input{
			Message: "Field name:",
			Help:    "The name of the field (e.g., ServerPort)",
		}
		if err := survey.AskOne(namePrompt, &field.Name, survey.WithValidator(survey.Required)); err != nil {
			return err
		}

		// Field type
		typeOptions := []string{
			"string", "int", "bool", "float64",
			"[]string (string array)", "[]int (int array)", "[]bool (bool array)",
			"map[string]string", "map[string]int", "map[string]bool",
			"nested struct", "custom",
		}
		typePrompt := &survey.Select{
			Message: "Field type:",
			Options: typeOptions,
		}
		var typeChoice string
		if err := survey.AskOne(typePrompt, &typeChoice); err != nil {
			return err
		}

		// Handle different type choices
		switch typeChoice {
		case "nested struct":
			field.IsNested = true
			nestedFields := []ConfigField{}
			fmt.Println("Define fields for nested struct:")
			if err := promptForConfigFields(&nestedFields); err != nil {
				return err
			}
			field.NestedFields = nestedFields
			field.Type = fmt.Sprintf("%sConfig", field.Name)
		case "[]string (string array)", "[]int (int array)", "[]bool (bool array)":
			field.IsArray = true
			field.Type = strings.Split(typeChoice, " ")[0] // Extract the actual type
		case "map[string]string", "map[string]int", "map[string]bool":
			field.IsMap = true
			field.Type = typeChoice
			parts := strings.SplitN(typeChoice, "]", 2)
			field.KeyType = "string"
			field.ValueType = parts[1]
		case "custom":
			customTypePrompt := &survey.Input{
				Message: "Enter custom type:",
				Help:    "The custom type (e.g., time.Duration)",
			}
			if err := survey.AskOne(customTypePrompt, &field.Type, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		default:
			field.Type = typeChoice
		}

		// Required field?
		requiredPrompt := &survey.Confirm{
			Message: "Is this field required?",
			Default: false,
		}
		if err := survey.AskOne(requiredPrompt, &field.IsRequired); err != nil {
			return err
		}

		// Default value (if not required)
		if !field.IsRequired {
			defaultValuePrompt := &survey.Input{
				Message: "Default value (leave empty for none):",
				Help:    "The default value for this field.",
			}
			if err := survey.AskOne(defaultValuePrompt, &field.DefaultValue); err != nil {
				return err
			}
		}

		// Description
		descPrompt := &survey.Input{
			Message: "Field description:",
			Help:    "A short description of what this field is used for.",
		}
		if err := survey.AskOne(descPrompt, &field.Description); err != nil {
			return err
		}

		// Add field to list
		*fields = append(*fields, field)
	}

	return nil
}

// generateStandaloneConfigFile generates a standalone config file
func generateStandaloneConfigFile(outputDir, configName string, options *ConfigOptions) error {
	configTmpl := `package config

// {{.ConfigName}}Config holds configuration settings
type {{.ConfigName}}Config struct {
	{{- range .Options.Fields}}
	{{template "configField" .}}
	{{- end}}
}

{{- range .Options.Fields}}
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
func (c *{{.ConfigName}}Config) Validate() error {
	// Add custom validation logic here
	return nil
}

// Setup implements modular.ConfigSetup (optional)
func (c *{{.ConfigName}}Config) Setup() error {
	// Perform any additional setup after config is loaded
	return nil
}
`

	fieldTmpl := `{{define "configField"}}{{.Name}} {{.Type}} {{template "tags" .}}{{if .Description}} // {{.Description}}{{end}}{{end}}`

	tagsTmpl := `{{define "tags"}}{{if or .IsRequired .DefaultValue (len .Tags)}} ` + "`" + `{{range $i, $tag := $.Tags}}{{if $i}}, {{end}}{{$tag}}:"{{$.Name | ToLower}}"{{end}}{{if .IsRequired}} required:"true"{{end}}{{if .DefaultValue}} default:"{{.DefaultValue}}"{{end}}{{if .Description}} desc:"{{.Description}}"{{end}}` + "`" + `{{end}}{{end}}`

	// Create function map for templates
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// Create and execute template
	tmpl, err := template.New("config").Funcs(funcMap).Parse(configTmpl + fieldTmpl + tagsTmpl)
	if err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	outputFile := filepath.Join(outputDir, strings.ToLower(configName)+"_config.go")
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Prepare data for template
	data := struct {
		ConfigName string
		Options    *ConfigOptions
	}{
		ConfigName: configName,
		Options:    options,
	}

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	return nil
}

// generateStandaloneSampleConfigs generates sample config files in the requested formats
func generateStandaloneSampleConfigs(outputDir, configName string, options *ConfigOptions) error {
	// Create samples directory
	samplesDir := filepath.Join(outputDir, "samples")
	if err := os.MkdirAll(samplesDir, 0755); err != nil {
		return fmt.Errorf("failed to create samples directory: %w", err)
	}

	// Generate samples in each requested format
	for _, format := range options.TagTypes {
		var fileExt, content string
		switch format {
		case "yaml":
			fileExt = "yaml"
			content, _ = generateYAMLSample(configName, options)
		case "json":
			fileExt = "json"
			content, _ = generateJSONSample(configName, options)
		case "toml":
			fileExt = "toml"
			content, _ = generateTOMLSample(configName, options)
		}

		if content != "" {
			outputFile := filepath.Join(samplesDir, fmt.Sprintf("config-sample.%s", fileExt))
			if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write %s sample file: %w", fileExt, err)
			}
		}
	}

	return nil
}

// generateYAMLSample generates a sample YAML config
func generateYAMLSample(configName string, options *ConfigOptions) (string, error) {
	// Sample config template for YAML
	yamlTmpl := `# {{.ConfigName}} Configuration
{{- range .Options.Fields}}
{{- if .Description}}
# {{.Description}}
{{- end}}
{{.Name | ToLower}}: {{template "yamlValue" .}}
{{- end}}
`

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

	// Create and execute template
	tmpl, err := template.New("yamlSample").Funcs(funcMap).Parse(yamlTmpl + yamlValueTmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse YAML template: %w", err)
	}

	var buf strings.Builder
	data := struct {
		ConfigName string
		Options    *ConfigOptions
	}{
		ConfigName: configName,
		Options:    options,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute YAML template: %w", err)
	}

	return buf.String(), nil
}

// generateJSONSample generates a sample JSON config
func generateJSONSample(configName string, options *ConfigOptions) (string, error) {
	// For brevity, I'm providing a simplified JSON generator
	var content strings.Builder
	content.WriteString("{\n")

	for i, field := range options.Fields {
		if i > 0 {
			content.WriteString(",\n")
		}

		fieldName := strings.ToLower(field.Name)
		var value string

		if field.DefaultValue != "" {
			value = field.DefaultValue
		} else {
			switch {
			case field.IsNested:
				value = "{}"
			case field.IsArray:
				value = "[]"
			case field.IsMap:
				value = "{}"
			case field.Type == "string":
				value = "\"example value\""
			case field.Type == "int":
				value = "42"
			case field.Type == "bool":
				value = "false"
			case field.Type == "float64":
				value = "3.14"
			default:
				value = "null"
			}
		}

		content.WriteString(fmt.Sprintf("  \"%s\": %s", fieldName, value))
	}

	content.WriteString("\n}")
	return content.String(), nil
}

// generateTOMLSample generates a sample TOML config
func generateTOMLSample(configName string, options *ConfigOptions) (string, error) {
	// For brevity, I'm providing a simplified TOML generator
	var content strings.Builder
	content.WriteString("# " + configName + " Configuration\n\n")

	for _, field := range options.Fields {
		if field.Description != "" {
			content.WriteString("# " + field.Description + "\n")
		}

		fieldName := strings.ToLower(field.Name)
		var value string

		if field.DefaultValue != "" {
			if field.Type == "string" {
				value = "\"" + field.DefaultValue + "\""
			} else {
				value = field.DefaultValue
			}
		} else {
			switch {
			case field.IsNested:
				// TOML uses sections for nested structs
				continue
			case field.IsArray:
				if field.Type == "[]string" {
					value = "[\"example\", \"values\"]"
				} else if field.Type == "[]int" {
					value = "[1, 2, 3]"
				} else if field.Type == "[]bool" {
					value = "[true, false]"
				}
			case field.IsMap:
				// Skip maps for now - TOML has a special syntax for them
				continue
			case field.Type == "string":
				value = "\"example value\""
			case field.Type == "int":
				value = "42"
			case field.Type == "bool":
				value = "false"
			case field.Type == "float64":
				value = "3.14"
			default:
				value = "# TODO: Set appropriate value for " + field.Type
				continue
			}
		}

		content.WriteString(fmt.Sprintf("%s = %s\n\n", fieldName, value))
	}

	return content.String(), nil
}
