// Package logmasker provides centralized log masking functionality for the Modular framework.
//
// This module wraps the Logger interface to provide configurable masking rules that can
// redact sensitive information from log output. It supports both field-based masking rules
// and value wrappers that can determine their own redaction behavior.
//
// # Features
//
// The logmasker module offers the following capabilities:
//   - Logger decorator that wraps any modular.Logger implementation
//   - Configurable field-based masking rules
//   - Regex pattern matching for sensitive data
//   - MaskableValue interface for self-determining value masking
//   - Multiple masking strategies (redact, partial mask, hash)
//   - Performance optimized for production use
//
// # Configuration
//
// The module can be configured through the LogMaskerConfig structure:
//
//	config := &LogMaskerConfig{
//	    Enabled: true,
//	    DefaultMaskStrategy: "redact",
//	    FieldRules: []FieldMaskingRule{
//	        {
//	            FieldName: "password",
//	            Strategy:  "redact",
//	        },
//	        {
//	            FieldName: "email",
//	            Strategy:  "partial",
//	            PartialConfig: &PartialMaskConfig{
//	                ShowFirst: 2,
//	                ShowLast:  2,
//	                MaskChar:  "*",
//	            },
//	        },
//	    },
//	    PatternRules: []PatternMaskingRule{
//	        {
//	            Pattern:  `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
//	            Strategy: "redact",
//	        },
//	    },
//	}
//
// # Usage Examples
//
// Basic usage as a service wrapper:
//
//	// Get the original logger
//	var originalLogger modular.Logger
//	app.GetService("logger", &originalLogger)
//
//	// Get the masking logger service
//	var maskingLogger modular.Logger
//	app.GetService("logmasker.logger", &maskingLogger)
//
//	// Use the masking logger
//	maskingLogger.Info("User login", "email", "user@example.com", "password", "secret123")
//	// Output: "User login" email="us*****.com" password="[REDACTED]"
package logmasker

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/CrisisTextLine/modular"
)

// ErrInvalidConfigType indicates the configuration type is incorrect for this module.
var ErrInvalidConfigType = errors.New("invalid config type for log masker")

const (
	// ServiceName is the name of the masking logger service.
	ServiceName = "logmasker.logger"

	// ModuleName is the name of the log masker module.
	ModuleName = "logmasker"
)

// MaskStrategy defines the type of masking to apply.
type MaskStrategy string

const (
	// MaskStrategyRedact replaces the entire value with "[REDACTED]".
	MaskStrategyRedact MaskStrategy = "redact"

	// MaskStrategyPartial shows only part of the value, masking the rest.
	MaskStrategyPartial MaskStrategy = "partial"

	// MaskStrategyHash replaces the value with a hash.
	MaskStrategyHash MaskStrategy = "hash"

	// MaskStrategyNone does not mask the value.
	MaskStrategyNone MaskStrategy = "none"
)

// MaskableValue is an interface that values can implement to control their own masking behavior.
// This allows for anytype-compatible value wrappers to determine if they should be redacted.
type MaskableValue interface {
	// ShouldMask returns true if this value should be masked in logs.
	ShouldMask() bool

	// GetMaskedValue returns the masked representation of this value.
	// If ShouldMask() returns false, this method may not be called.
	GetMaskedValue() any

	// GetMaskStrategy returns the preferred masking strategy for this value.
	// Can return an empty string to use the default strategy.
	GetMaskStrategy() MaskStrategy
}

// FieldMaskingRule defines masking rules for specific field names.
type FieldMaskingRule struct {
	// FieldName is the exact field name to match (case-sensitive).
	FieldName string `yaml:"fieldName" json:"fieldName" desc:"Field name to mask"`

	// Strategy defines how to mask this field.
	Strategy MaskStrategy `yaml:"strategy" json:"strategy" desc:"Masking strategy to use"`

	// PartialConfig provides configuration for partial masking.
	PartialConfig *PartialMaskConfig `yaml:"partialConfig,omitempty" json:"partialConfig,omitempty" desc:"Configuration for partial masking"`
}

// PatternMaskingRule defines masking rules based on regex patterns.
type PatternMaskingRule struct {
	// Pattern is the regular expression to match against string values.
	Pattern string `yaml:"pattern" json:"pattern" desc:"Regular expression pattern to match"`

	// Strategy defines how to mask values matching this pattern.
	Strategy MaskStrategy `yaml:"strategy" json:"strategy" desc:"Masking strategy to use"`

	// PartialConfig provides configuration for partial masking.
	PartialConfig *PartialMaskConfig `yaml:"partialConfig,omitempty" json:"partialConfig,omitempty" desc:"Configuration for partial masking"`

	// compiled is the compiled regex (not exposed in config).
	compiled *regexp.Regexp
}

// PartialMaskConfig defines how to partially mask a value.
type PartialMaskConfig struct {
	// ShowFirst is the number of characters to show at the beginning.
	ShowFirst int `yaml:"showFirst" json:"showFirst" default:"0" desc:"Number of characters to show at start"`

	// ShowLast is the number of characters to show at the end.
	ShowLast int `yaml:"showLast" json:"showLast" default:"0" desc:"Number of characters to show at end"`

	// MaskChar is the character to use for masking.
	MaskChar string `yaml:"maskChar" json:"maskChar" default:"*" desc:"Character to use for masking"`

	// MinLength is the minimum length before applying partial masking.
	MinLength int `yaml:"minLength" json:"minLength" default:"4" desc:"Minimum length before applying partial masking"`
}

// LogMaskerConfig defines the configuration for the log masking module.
type LogMaskerConfig struct {
	// Enabled controls whether log masking is active.
	Enabled bool `yaml:"enabled" json:"enabled" default:"true" desc:"Enable log masking"`

	// DefaultMaskStrategy is used when no specific rule matches.
	DefaultMaskStrategy MaskStrategy `yaml:"defaultMaskStrategy" json:"defaultMaskStrategy" default:"redact" desc:"Default masking strategy"`

	// FieldRules defines masking rules for specific field names.
	FieldRules []FieldMaskingRule `yaml:"fieldRules" json:"fieldRules" desc:"Field-based masking rules"`

	// PatternRules defines masking rules based on regex patterns.
	PatternRules []PatternMaskingRule `yaml:"patternRules" json:"patternRules" desc:"Pattern-based masking rules"`

	// DefaultPartialConfig provides default settings for partial masking.
	DefaultPartialConfig PartialMaskConfig `yaml:"defaultPartialConfig" json:"defaultPartialConfig" desc:"Default partial masking configuration"`
}

// LogMaskerModule implements the modular.Module interface to provide log masking functionality.
type LogMaskerModule struct {
	config           *LogMaskerConfig
	originalLogger   modular.Logger
	compiledPatterns []*PatternMaskingRule
}

// NewModule creates a new log masker module instance.
func NewModule() *LogMaskerModule {
	return &LogMaskerModule{}
}

// Name returns the module name.
func (m *LogMaskerModule) Name() string {
	return ModuleName
}

// RegisterConfig registers the module's configuration.
func (m *LogMaskerModule) RegisterConfig(app modular.Application) error {
	defaultConfig := &LogMaskerConfig{
		Enabled:             true,
		DefaultMaskStrategy: MaskStrategyRedact,
		FieldRules: []FieldMaskingRule{
			{
				FieldName: "password",
				Strategy:  MaskStrategyRedact,
			},
			{
				FieldName: "token",
				Strategy:  MaskStrategyRedact,
			},
			{
				FieldName: "secret",
				Strategy:  MaskStrategyRedact,
			},
			{
				FieldName: "key",
				Strategy:  MaskStrategyRedact,
			},
			{
				FieldName: "email",
				Strategy:  MaskStrategyPartial,
				PartialConfig: &PartialMaskConfig{
					ShowFirst: 2,
					ShowLast:  2,
					MaskChar:  "*",
					MinLength: 4,
				},
			},
		},
		PatternRules: []PatternMaskingRule{
			{
				Pattern:  `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`, // Credit card numbers
				Strategy: MaskStrategyRedact,
			},
			{
				Pattern:  `\b\d{3}-\d{2}-\d{4}\b`, // SSN format
				Strategy: MaskStrategyRedact,
			},
		},
		DefaultPartialConfig: PartialMaskConfig{
			ShowFirst: 2,
			ShowLast:  2,
			MaskChar:  "*",
			MinLength: 4,
		},
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the module.
func (m *LogMaskerModule) Init(app modular.Application) error {
	// Get configuration
	configProvider, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get log masker config: %w", err)
	}

	config, ok := configProvider.GetConfig().(*LogMaskerConfig)
	if !ok {
		return fmt.Errorf("%w", ErrInvalidConfigType)
	}

	m.config = config

	// Get the original logger
	if err := app.GetService("logger", &m.originalLogger); err != nil {
		return fmt.Errorf("failed to get logger service: %w", err)
	}

	// Compile regex patterns
	m.compiledPatterns = make([]*PatternMaskingRule, len(config.PatternRules))
	for i, rule := range config.PatternRules {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile pattern '%s': %w", rule.Pattern, err)
		}

		// Create a copy of the rule with compiled regex
		compiledRule := rule
		compiledRule.compiled = compiled
		m.compiledPatterns[i] = &compiledRule
	}

	// Register the masking logger service using the decorator pattern
	maskingLogger := &MaskingLogger{
		BaseLoggerDecorator: modular.NewBaseLoggerDecorator(m.originalLogger),
		module:              m,
	}
	if err := app.RegisterService(ServiceName, maskingLogger); err != nil {
		return fmt.Errorf("failed to register masking logger service: %w", err)
	}

	return nil
}

// Dependencies returns the list of module dependencies.
func (m *LogMaskerModule) Dependencies() []string {
	return nil // No module dependencies, but requires logger service
}

// ProvidesServices declares what services this module provides.
func (m *LogMaskerModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Masking logger that wraps the original logger with redaction capabilities",
			Instance:    nil, // Will be registered in Init()
		},
	}
}

// MaskingLogger implements modular.LoggerDecorator with masking capabilities.
// It extends BaseLoggerDecorator to leverage the framework's decorator infrastructure.
type MaskingLogger struct {
	*modular.BaseLoggerDecorator
	module *LogMaskerModule
}

// Info logs an informational message with masking applied to arguments.
func (l *MaskingLogger) Info(msg string, args ...any) {
	if !l.module.config.Enabled {
		l.BaseLoggerDecorator.Info(msg, args...)
		return
	}

	maskedArgs := l.maskArgs(args...)
	l.BaseLoggerDecorator.Info(msg, maskedArgs...)
}

// Error logs an error message with masking applied to arguments.
func (l *MaskingLogger) Error(msg string, args ...any) {
	if !l.module.config.Enabled {
		l.BaseLoggerDecorator.Error(msg, args...)
		return
	}

	maskedArgs := l.maskArgs(args...)
	l.BaseLoggerDecorator.Error(msg, maskedArgs...)
}

// Warn logs a warning message with masking applied to arguments.
func (l *MaskingLogger) Warn(msg string, args ...any) {
	if !l.module.config.Enabled {
		l.BaseLoggerDecorator.Warn(msg, args...)
		return
	}

	maskedArgs := l.maskArgs(args...)
	l.BaseLoggerDecorator.Warn(msg, maskedArgs...)
}

// Debug logs a debug message with masking applied to arguments.
func (l *MaskingLogger) Debug(msg string, args ...any) {
	if !l.module.config.Enabled {
		l.BaseLoggerDecorator.Debug(msg, args...)
		return
	}

	maskedArgs := l.maskArgs(args...)
	l.BaseLoggerDecorator.Debug(msg, maskedArgs...)
}

// maskArgs applies masking rules to key-value pairs in the arguments.
func (l *MaskingLogger) maskArgs(args ...any) []any {
	if len(args) == 0 {
		return args
	}

	result := make([]any, len(args))

	// Process key-value pairs
	for i := 0; i < len(args); i += 2 {
		// Copy the key
		result[i] = args[i]

		// Process the value if it exists
		if i+1 < len(args) {
			value := args[i+1]

			// Check if value implements MaskableValue
			if maskable, ok := value.(MaskableValue); ok {
				if maskable.ShouldMask() {
					result[i+1] = maskable.GetMaskedValue()
				} else {
					result[i+1] = value
				}
				continue
			}

			// Apply field-based rules
			if i < len(args) {
				if keyStr, ok := args[i].(string); ok {
					result[i+1] = l.applyMaskingRules(keyStr, value)
				} else {
					result[i+1] = value
				}
			} else {
				result[i+1] = value
			}
		}
	}

	return result
}

// applyMaskingRules applies the configured masking rules to a value.
func (l *MaskingLogger) applyMaskingRules(fieldName string, value any) any {
	// First check field rules
	for _, rule := range l.module.config.FieldRules {
		if rule.FieldName == fieldName {
			return l.applyMaskStrategy(value, rule.Strategy, rule.PartialConfig)
		}
	}

	// Then check pattern rules for string values
	if strValue, ok := value.(string); ok {
		for _, rule := range l.module.compiledPatterns {
			if rule.compiled.MatchString(strValue) {
				return l.applyMaskStrategy(value, rule.Strategy, rule.PartialConfig)
			}
		}
	}

	return value
}

// applyMaskStrategy applies a specific masking strategy to a value.
func (l *MaskingLogger) applyMaskStrategy(value any, strategy MaskStrategy, partialConfig *PartialMaskConfig) any {
	switch strategy {
	case MaskStrategyRedact:
		return "[REDACTED]"

	case MaskStrategyPartial:
		if strValue, ok := value.(string); ok {
			config := partialConfig
			if config == nil {
				config = &l.module.config.DefaultPartialConfig
			}
			return l.partialMask(strValue, config)
		}
		return "[REDACTED]" // Fallback for non-string values

	case MaskStrategyHash:
		return fmt.Sprintf("[HASH:%x]", fmt.Sprintf("%v", value))

	case MaskStrategyNone:
		return value

	default:
		return l.applyMaskStrategy(value, l.module.config.DefaultMaskStrategy, nil)
	}
}

// partialMask applies partial masking to a string value.
func (l *MaskingLogger) partialMask(value string, config *PartialMaskConfig) string {
	if len(value) < config.MinLength {
		return value
	}

	showFirst := config.ShowFirst
	showLast := config.ShowLast

	// Ensure we don't show more characters than the string length
	if showFirst+showLast >= len(value) {
		return value
	}

	maskChar := config.MaskChar
	if maskChar == "" {
		maskChar = "*"
	}

	first := value[:showFirst]
	last := ""
	if showLast > 0 {
		last = value[len(value)-showLast:]
	}

	maskLength := len(value) - showFirst - showLast
	mask := strings.Repeat(maskChar, maskLength)

	return first + mask + last
}
