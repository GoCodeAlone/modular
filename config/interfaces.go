// Package config defines interfaces for configuration management services
package config

import (
	"context"
	"time"
)

// ConfigLoader defines the interface for loading configuration from various sources
type ConfigLoader interface {
	// Load loads configuration from all configured sources and applies validation
	Load(ctx context.Context, config interface{}) error

	// Reload reloads configuration from sources, applying hot-reload logic where supported
	Reload(ctx context.Context, config interface{}) error

	// Validate validates the given configuration against defined rules and schemas
	Validate(ctx context.Context, config interface{}) error

	// GetProvenance returns field-level provenance information for configuration
	GetProvenance(ctx context.Context, fieldPath string) (*FieldProvenance, error)

	// GetSources returns information about all configured configuration sources
	GetSources(ctx context.Context) ([]*ConfigSource, error)
}

// ConfigValidator defines the interface for configuration validation services
type ConfigValidator interface {
	// ValidateStruct validates an entire configuration struct
	ValidateStruct(ctx context.Context, config interface{}) error

	// ValidateField validates a specific field with the given value
	ValidateField(ctx context.Context, fieldPath string, value interface{}) error

	// GetValidationRules returns validation rules for the given configuration type
	GetValidationRules(ctx context.Context, configType string) ([]*ValidationRule, error)
}

// ConfigReloader defines the interface for configuration hot-reload functionality
type ConfigReloader interface {
	// StartWatch starts watching configuration sources for changes
	StartWatch(ctx context.Context, callback ReloadCallback) error

	// StopWatch stops watching configuration sources
	StopWatch(ctx context.Context) error

	// IsWatching returns true if currently watching for configuration changes
	IsWatching() bool
}

// FieldProvenance represents provenance information for a configuration field
type FieldProvenance struct {
	FieldPath    string            `json:"field_path"`
	Source       string            `json:"source"`        // e.g., "env", "yaml", "default"
	SourceDetail string            `json:"source_detail"` // e.g., "ENV_VAR_NAME", "config.yaml:line:23"
	Value        interface{}       `json:"value"`
	Timestamp    time.Time         `json:"timestamp"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ConfigSource represents a configuration source
type ConfigSource struct {
	Name       string            `json:"name"`     // e.g., "environment", "yaml-file"
	Type       string            `json:"type"`     // e.g., "env", "yaml", "json", "toml"
	Location   string            `json:"location"` // file path, URL, etc.
	Priority   int               `json:"priority"` // higher priority overrides lower
	Loaded     bool              `json:"loaded"`   // true if successfully loaded
	LastLoaded *time.Time        `json:"last_loaded,omitempty"`
	Error      string            `json:"error,omitempty"` // error message if loading failed
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ValidationRule represents a validation rule for configuration fields
type ValidationRule struct {
	FieldPath  string                 `json:"field_path"`
	RuleType   string                 `json:"rule_type"`  // e.g., "required", "min", "max", "pattern"
	Parameters map[string]interface{} `json:"parameters"` // rule-specific parameters
	Message    string                 `json:"message"`    // custom error message
	Severity   string                 `json:"severity"`   // "error", "warning"
}

// ReloadCallback is called when configuration changes are detected
type ReloadCallback func(ctx context.Context, changes []*ConfigChange) error

// ConfigChange represents a change in configuration
type ConfigChange struct {
	FieldPath string      `json:"field_path"`
	OldValue  interface{} `json:"old_value"`
	NewValue  interface{} `json:"new_value"`
	Source    string      `json:"source"`
	Timestamp time.Time   `json:"timestamp"`
}
