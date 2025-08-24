package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// Use the SurveyIO from survey_stdio.go
var configSurveyIO = DefaultSurveyIO

// NewGenerateConfigCommand creates a new 'generate config' command
func NewGenerateConfigCommand() *cobra.Command {
	var outputDir string
	var configName string
	var fileFormats []string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Generate a Go configuration struct and sample config files",
		Long: `Generate a configuration struct for your Go application, along with sample configuration files.
Supported formats include YAML, JSON, and TOML.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Collect configuration information
			configOptions := &ConfigOptions{
				Name:           configName,
				TagTypes:       fileFormats,
				GenerateSample: true,
				Fields:         []ConfigField{},
			}

			// If config name is not provided, prompt for it
			if configOptions.Name == "" {
				namePrompt := &survey.Input{
					Message: "What is the name of your configuration struct?",
					Default: "Config",
					Help:    "This will be the name of your Go struct (e.g., AppConfig).",
				}
				if err := survey.AskOne(namePrompt, &configOptions.Name, configSurveyIO.WithStdio()); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
			}

			// If file formats are not provided, prompt for them
			if len(configOptions.TagTypes) == 0 {
				formatPrompt := &survey.MultiSelect{
					Message: "Which configuration formats do you want to support?",
					Options: []string{"yaml", "json", "toml", "env"},
					Default: []string{"yaml"},
					Help:    "Select one or more formats for your configuration files.",
				}
				if err := survey.AskOne(formatPrompt, &configOptions.TagTypes, configSurveyIO.WithStdio()); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
					os.Exit(1)
				}
			}

			// If no formats were selected, default to YAML
			if len(configOptions.TagTypes) == 0 {
				configOptions.TagTypes = []string{"yaml"}
			}

			// Prompt for configuration fields
			if err := promptForConfigFields(configOptions); err != nil {
				fmt.Fprintf(os.Stderr, "Error collecting field information: %s\n", err)
				os.Exit(1)
			}

			// Generate the config struct file
			if err := GenerateStandaloneConfigFile(outputDir, configOptions); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating config struct: %s\n", err)
				os.Exit(1)
			}

			// Generate sample configuration files
			if configOptions.GenerateSample {
				if err := GenerateStandaloneSampleConfigs(outputDir, configOptions); err != nil {
					fmt.Fprintf(os.Stderr, "Error generating sample configs: %s\n", err)
					os.Exit(1)
				}
			}

			fmt.Printf("Successfully generated configuration files in %s\n", outputDir)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "Directory where the config files will be generated")
	cmd.Flags().StringVarP(&configName, "name", "n", "", "Name of the configuration struct (e.g., AppConfig)")
	cmd.Flags().StringSliceVarP(&fileFormats, "formats", "f", []string{}, "File formats to support (yaml, json, toml, env)")

	return cmd
}

// promptForConfigFields collects information about configuration fields
func promptForConfigFields(options *ConfigOptions) error {
	// Ask if sample configs should be generated
	samplePrompt := &survey.Confirm{
		Message: "Generate sample configuration files?",
		Default: true,
		Help:    "If yes, sample configuration files will be generated in the selected formats.",
	}
	if err := survey.AskOne(samplePrompt, &options.GenerateSample, configSurveyIO.WithStdio()); err != nil {
		return fmt.Errorf("failed to get sample config preference: %w", err)
	}

	// Collect field information
	options.Fields = []ConfigField{}
	addFields := true

	for addFields {
		field := ConfigField{}

		// Ask for the field name
		namePrompt := &survey.Input{
			Message: "Field name (CamelCase):",
			Help:    "The name of the configuration field (e.g., ServerAddress)",
		}
		if err := survey.AskOne(namePrompt, &field.Name, survey.WithValidator(survey.Required), configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get field name: %w", err)
		}

		// Ask for the field type
		typePrompt := &survey.Select{
			Message: "Field type:",
			Options: []string{"string", "int", "bool", "float64", "[]string", "[]int", "map[string]string", "struct (nested)"},
			Default: "string",
			Help:    "The data type of this configuration field.",
		}

		var fieldType string
		if err := survey.AskOne(typePrompt, &fieldType, configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get field type: %w", err)
		}

		// Set field type and additional properties based on selection
		switch fieldType {
		case "struct (nested)":
			field.IsNested = true
			field.Type = field.Name + "Config" // Create a type name based on the field name
			field.Tags = options.TagTypes

			// Ask if we should define the nested struct fields now
			defineNested := false
			nestedPrompt := &survey.Confirm{
				Message: "Do you want to define the nested struct fields now?",
				Default: true,
				Help:    "If yes, you'll be prompted to add fields to the nested struct.",
			}
			if err := survey.AskOne(nestedPrompt, &defineNested, configSurveyIO.WithStdio()); err != nil {
				return fmt.Errorf("failed to get nested struct preference: %w", err)
			}

			if defineNested {
				// Create a new options instance for the nested fields
				nestedOptions := &ConfigOptions{
					Fields:   []ConfigField{},
					TagTypes: options.TagTypes,
				}

				// Reuse the promptForConfigFields function but without the sample generation prompt
				addNestedFields := true
				for addNestedFields {
					nestedField := ConfigField{}

					// Ask for the nested field name
					nestedNamePrompt := &survey.Input{
						Message: fmt.Sprintf("Nested field name for %s:", field.Type),
						Help:    "The name of the nested configuration field.",
					}
					if err := survey.AskOne(nestedNamePrompt, &nestedField.Name, survey.WithValidator(survey.Required), configSurveyIO.WithStdio()); err != nil {
						return fmt.Errorf("failed to get nested field name: %w", err)
					}

					// Ask for the nested field type
					nestedTypePrompt := &survey.Select{
						Message: "Nested field type:",
						Options: []string{"string", "int", "bool", "float64", "[]string", "[]int", "map[string]string"},
						Default: "string",
						Help:    "The data type of this nested configuration field.",
					}

					var nestedFieldType string
					if err := survey.AskOne(nestedTypePrompt, &nestedFieldType, configSurveyIO.WithStdio()); err != nil {
						return fmt.Errorf("failed to get nested field type: %w", err)
					}

					// Set nested field type
					nestedField.Type = nestedFieldType
					nestedField.Tags = options.TagTypes

					// Set additional properties for arrays and maps
					if strings.HasPrefix(nestedFieldType, "[]") {
						nestedField.IsArray = true
					} else if strings.HasPrefix(nestedFieldType, "map[") {
						nestedField.IsMap = true
						parts := strings.Split(strings.Trim(nestedFieldType, "map[]"), "]")
						if len(parts) >= 2 {
							nestedField.KeyType = strings.TrimPrefix(parts[0], "[")
							nestedField.ValueType = parts[1]
						}
					}

					// Ask for a description
					descPrompt := &survey.Input{
						Message: "Description:",
						Help:    "A brief description of what this nested field is used for.",
					}
					if err := survey.AskOne(descPrompt, &nestedField.Description, configSurveyIO.WithStdio()); err != nil {
						return fmt.Errorf("failed to get nested field description: %w", err)
					}

					// Add the nested field
					nestedOptions.Fields = append(nestedOptions.Fields, nestedField)

					// Ask if more nested fields should be added
					moreNestedPrompt := &survey.Confirm{
						Message: fmt.Sprintf("Add another field to %s?", field.Type),
						Default: true,
						Help:    "If yes, you'll be prompted for another nested field.",
					}
					if err := survey.AskOne(moreNestedPrompt, &addNestedFields, configSurveyIO.WithStdio()); err != nil {
						return fmt.Errorf("failed to get add more nested fields preference: %w", err)
					}
				}

				// Set the nested fields on the parent field
				field.NestedFields = nestedOptions.Fields
			}
		case "[]string", "[]int", "[]bool":
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

		// Set the tags based on the selected formats
		field.Tags = options.TagTypes

		// Ask if this field is required
		requiredPrompt := &survey.Confirm{
			Message: "Is this field required?",
			Default: false,
			Help:    "If yes, validation will ensure this field is provided.",
		}
		if err := survey.AskOne(requiredPrompt, &field.IsRequired, configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get add another field preference: %w", err)
		}

		// Ask for a default value
		defaultPrompt := &survey.Input{
			Message: "Default value (leave empty for none):",
			Help:    "The default value for this field, if any.",
		}
		if err := survey.AskOne(defaultPrompt, &field.DefaultValue, configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get field required preference: %w", err)
		}

		// Ask for a description
		descPrompt := &survey.Input{
			Message: "Description:",
			Help:    "A brief description of what this field is used for.",
		}
		if err := survey.AskOne(descPrompt, &field.Description, configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get field default value: %w", err)
		}

		// Add the field
		options.Fields = append(options.Fields, field)

		// Ask if more fields should be added
		morePrompt := &survey.Confirm{
			Message: "Add another field?",
			Default: true,
			Help:    "If yes, you'll be prompted for another configuration field.",
		}
		if err := survey.AskOne(morePrompt, &addFields, configSurveyIO.WithStdio()); err != nil {
			return fmt.Errorf("failed to get field description: %w", err)
		}
	}

	return nil
}

// GenerateStandaloneConfigFile generates a Go file with the config struct
func GenerateStandaloneConfigFile(outputDir string, options *ConfigOptions) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create function map for the template
	funcMap := template.FuncMap{
		"ToLowerF": ToLowerF,
	}

	// Parse the template from the embedded template string
	configTemplate, err := template.New("config").Funcs(funcMap).Parse(configTemplateText)
	if err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	// Execute the template
	var content bytes.Buffer
	data := map[string]interface{}{
		"ConfigName": options.Name,
		"Options":    options,
	}

	// Execute the main template
	if err := configTemplate.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute config template: %w", err)
	}

	// Write the generated config to a file
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.go", strings.ToLower(options.Name)))
	if err := os.WriteFile(outputFile, content.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GenerateStandaloneSampleConfigs generates sample configuration files for the selected formats
func GenerateStandaloneSampleConfigs(outputDir string, options *ConfigOptions) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate sample files for each format
	for _, format := range options.TagTypes {
		switch format {
		case "yaml":
			if err := generateYAMLSample(outputDir, options); err != nil {
				return fmt.Errorf("failed to generate YAML sample: %w", err)
			}
		case "json":
			if err := generateJSONSample(outputDir, options); err != nil {
				return fmt.Errorf("failed to generate JSON sample: %w", err)
			}
		case "toml":
			if err := generateTOMLSample(outputDir, options); err != nil {
				return fmt.Errorf("failed to generate TOML sample: %w", err)
			}
		case "env":
			// Skip .env sample for now as it's more complex
		}
	}

	return nil
}

// generateYAMLSample generates a sample YAML configuration file
func generateYAMLSample(outputDir string, options *ConfigOptions) error {
	// Create function map for the template
	funcMap := template.FuncMap{
		"ToLowerF": ToLowerF,
	}

	// Create template for YAML
	yamlTemplate, err := template.New("yaml").Funcs(funcMap).Parse(yamlTemplateText)
	if err != nil {
		return fmt.Errorf("failed to parse YAML template: %w", err)
	}

	// Execute the template
	var content bytes.Buffer
	if err := yamlTemplate.Execute(&content, options); err != nil {
		return fmt.Errorf("failed to execute YAML template: %w", err)
	}

	// Write the sample YAML to a file
	outputFile := filepath.Join(outputDir, "config-sample.yaml")
	if err := os.WriteFile(outputFile, content.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write YAML sample: %w", err)
	}

	return nil
}

// generateJSONSample generates a sample JSON configuration file
func generateJSONSample(outputDir string, options *ConfigOptions) error {
	// Create function map for the template
	funcMap := template.FuncMap{
		"ToLowerF": ToLowerF,
	}

	// Create template for JSON
	jsonTemplate, err := template.New("json").Funcs(funcMap).Parse(jsonTemplateText)
	if err != nil {
		return fmt.Errorf("failed to parse JSON template: %w", err)
	}

	// Execute the template
	var content bytes.Buffer
	if err := jsonTemplate.Execute(&content, options); err != nil {
		return fmt.Errorf("failed to execute JSON template: %w", err)
	}

	// Write the sample JSON to a file
	outputFile := filepath.Join(outputDir, "config-sample.json")
	if err := os.WriteFile(outputFile, content.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write JSON sample: %w", err)
	}

	return nil
}

// generateTOMLSample generates a sample TOML configuration file
func generateTOMLSample(outputDir string, options *ConfigOptions) error {
	filePath := filepath.Join(outputDir, "config-sample.toml")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create TOML sample file: %w", err)
	}
	defer file.Close()

	// Use standard template functions
	funcMap := template.FuncMap{
		"ToLowerF": ToLowerF,
	}

	tmpl, err := template.New("toml").Funcs(funcMap).Parse(tomlTemplateText)
	if err != nil {
		return fmt.Errorf("failed to parse TOML template: %w", err)
	}

	if err := tmpl.Execute(file, options); err != nil {
		return fmt.Errorf("failed to execute TOML template: %w", err)
	}

	return nil
}

// Template for generating a config struct file
const configTemplateText = `package config

// {{.ConfigName}} defines the application configuration structure
type {{.ConfigName}} struct {
{{- range $field := .Options.Fields}}
	{{if $field.Description}}// {{$field.Description}}{{end}}
	{{$field.Name}} {{$field.Type}} ` + "`" + `{{range $i, $tag := $field.Tags}}{{if $i}} {{end}}{{$tag}}:"{{$field.Name | ToLowerF}}"{{end}}{{if $field.IsRequired}} validate:"required"{{end}}{{if $field.DefaultValue}} default:"{{$field.DefaultValue}}"{{end}}` + "`" + `
{{- end}}
}

{{- range $field := .Options.Fields}}
{{- if $field.IsNested}}

// {{$field.Type}} defines the nested configuration for {{$field.Name}}
type {{$field.Type}} struct {
{{- range $nested := $field.NestedFields}}
	{{if $nested.Description}}// {{$nested.Description}}{{end}}
	{{$nested.Name}} {{$nested.Type}} ` + "`" + `{{range $i, $tag := $nested.Tags}}{{if $i}} {{end}}{{$tag}}:"{{$nested.Name | ToLowerF}}"{{end}}{{if $nested.IsRequired}} validate:"required"{{end}}{{if $nested.DefaultValue}} default:"{{$nested.DefaultValue}}"{{end}}` + "`" + `
{{- end}}
}
{{- end}}
{{- end}}

// Validate validates the configuration
func (c *{{.ConfigName}}) Validate() error {
	// Add custom validation logic here
	return nil
}
`

// Template for generating a sample YAML configuration file
const yamlTemplateText = `# Sample configuration
{{- range $field := .Fields}}
{{- if $field.Description}}
# {{$field.Description}}
{{- end}}
{{$field.Name | ToLowerF}}: {{if $field.IsNested}}
{{- range $nested := $field.NestedFields}}
  {{$nested.Name | ToLowerF}}: {{if $nested.DefaultValue}}{{$nested.DefaultValue}}{{else}}{{if eq $nested.Type "string"}}"example"{{else if eq $nested.Type "int"}}0{{else if eq $nested.Type "bool"}}false{{else if eq $nested.Type "float64"}}0.0{{else if $nested.IsArray}}[]{{else if $nested.IsMap}}{}{{else}}""{{end}}{{end}}
{{- end}}
{{- else if $field.DefaultValue}}
  {{$field.DefaultValue}}
{{- else if eq $field.Type "string"}}
  "example string"
{{- else if eq $field.Type "int"}}
  0
{{- else if eq $field.Type "bool"}}
  false
{{- else if eq $field.Type "float64"}}
  0.0
{{- else if $field.IsArray}}
  []
{{- else if $field.IsMap}}
  {}
{{- else}}
  # Set a value appropriate for the type {{$field.Type}}
{{- end}}
{{- end}}
`

// Template for generating a sample JSON configuration file
const jsonTemplateText = `{
{{- range $i, $field := .Fields}}
  {{- if $i}},{{end}}
  "{{$field.Name | ToLowerF}}": {{if $field.IsNested}}{
    {{- range $j, $nested := $field.NestedFields}}
    {{- if $j}},{{end}}
    "{{$nested.Name | ToLowerF}}": {{if $nested.DefaultValue}}{{$nested.DefaultValue}}{{else}}{{if eq $nested.Type "string"}}"example"{{else if eq $nested.Type "int"}}0{{else if eq $nested.Type "bool"}}false{{else if eq $nested.Type "float64"}}0.0{{else if $nested.IsArray}}[]{{else if $nested.IsMap}}{}{{else}}""{{end}}{{end}}
    {{- end}}
  }{{else if $field.DefaultValue}}{{$field.DefaultValue}}{{else if eq $field.Type "string"}}"example string"{{else if eq $field.Type "int"}}0{{else if eq $field.Type "bool"}}false{{else if eq $field.Type "float64"}}0.0{{else if $field.IsArray}}[]{{else if $field.IsMap}}{}{{else}}null{{end}}
{{- end}}
}
`

// Template for generating a sample TOML configuration file
const tomlTemplateText = `# Sample configuration
{{- range $field := .Fields}}
{{- if $field.Description}}
# {{$field.Description}}
{{- end}}
{{- if $field.DefaultValue}}
  {{- if eq $field.Type "string"}}
{{$field.Name | ToLowerF}} = "{{$field.DefaultValue}}"
  {{- else}}
# {{$field.Name | ToLowerF}} = {{$field.DefaultValue}} # Default value for type {{$field.Type}} - Uncomment and format correctly
  {{- end}}
{{- else if $field.IsNested}}
# {{$field.Name | ToLowerF}} = # Nested structure, define below or inline
# Example:
# [{{$field.Name | ToLowerF}}]
{{- range $nested := $field.NestedFields}}
#   {{$nested.Name | ToLowerF}} = {{if eq $nested.Type "string"}}"nested_example"{{else if eq $nested.Type "int"}}0{{else if eq $nested.Type "bool"}}false{{else if eq $nested.Type "float64"}}0.0{{else}}""{{end}} # Example for nested field {{$nested.Name}}
{{- end}}
{{- else if eq $field.Type "string"}}
{{$field.Name | ToLowerF}} = "example string"
{{- else if eq $field.Type "int"}}
{{$field.Name | ToLowerF}} = 0
{{- else if eq $field.Type "bool"}}
{{$field.Name | ToLowerF}} = false
{{- else if eq $field.Type "float64"}}
{{$field.Name | ToLowerF}} = 0.0
{{- else if $field.IsArray}}
{{$field.Name | ToLowerF}} = [] # Example: ["item1", "item2"] or [1, 2]
{{- else if $field.IsMap}}
{{$field.Name | ToLowerF}} = {} # Example: { key1 = "value1", key2 = "value2" }
{{- else}}
{{$field.Name | ToLowerF}} = "" # Set a value appropriate for the type {{$field.Type}}
{{- end}}
{{- end}}
`

// ToLowerF is a function for templates to convert a string to lowercase
func ToLowerF(s string) string {
	return strings.ToLower(s)
}
