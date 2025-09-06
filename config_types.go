package modular

import (
	"time"
)

// ConfigurationField represents a single field in a configuration structure
type ConfigurationField struct {
	// FieldName is the name of the configuration field
	FieldName string

	// Type is the Go type of the field (string, int, bool, etc.)
	Type string

	// DefaultValue is the default value for this field (optional)
	DefaultValue interface{}

	// Required indicates if this field must be provided
	Required bool

	// Description provides human-readable documentation for this field
	Description string

	// Dynamic indicates if this field supports hot-reload
	Dynamic bool

	// Provenance tracks which feeder provided the value for this field
	Provenance *FieldProvenance

	// Path is the full path to this field (e.g., "database.connections.primary.host")
	Path string

	// Tags contains struct tags associated with this field
	Tags map[string]string
}

// FieldProvenance tracks the source of a configuration field value
type FieldProvenance struct {
	// FeederID identifies which feeder provided this value
	FeederID string

	// FeederType is the type of feeder (env, file, programmatic, etc.)
	FeederType string

	// Source contains source-specific information (file path, env var name, etc.)
	Source string

	// Timestamp records when this value was set
	Timestamp time.Time

	// Redacted indicates if the value was redacted for security
	Redacted bool

	// RedactedValue is the redacted representation (e.g., "***")
	RedactedValue string
}

// ConfigurationSchema represents metadata about a module's configuration structure
type ConfigurationSchema struct {
	// ModuleName is the name of the module this schema belongs to
	ModuleName string

	// Version is the schema version
	Version string

	// Fields contains metadata for all configuration fields
	Fields []ConfigurationField

	// RequiredFields lists the names of required fields
	RequiredFields []string

	// DynamicFields lists the names of fields that support hot-reload
	DynamicFields []string

	// ValidationRules contains custom validation logic description
	ValidationRules []ValidationRule
}

// ValidationRule represents a custom validation rule for configuration
type ValidationRule struct {
	// RuleName is the name of the validation rule
	RuleName string

	// Description describes what this rule validates
	Description string

	// Fields lists the fields this rule applies to
	Fields []string

	// RuleType indicates the type of validation (type, range, regex, custom, etc.)
	RuleType string

	// Parameters contains rule-specific parameters
	Parameters map[string]interface{}
}
