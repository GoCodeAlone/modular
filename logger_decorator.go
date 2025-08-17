package modular

import (
	"fmt"
	"strings"
)

// LoggerDecorator defines the interface for decorating loggers.
// Decorators wrap loggers to add additional functionality without
// modifying the core logger implementation.
type LoggerDecorator interface {
	Logger

	// GetInnerLogger returns the wrapped logger
	GetInnerLogger() Logger
}

// BaseLoggerDecorator provides a foundation for logger decorators.
// It implements LoggerDecorator by forwarding all calls to the wrapped logger.
type BaseLoggerDecorator struct {
	inner Logger
}

// NewBaseLoggerDecorator creates a new base decorator wrapping the given logger.
func NewBaseLoggerDecorator(inner Logger) *BaseLoggerDecorator {
	return &BaseLoggerDecorator{inner: inner}
}

// GetInnerLogger returns the wrapped logger
func (d *BaseLoggerDecorator) GetInnerLogger() Logger {
	return d.inner
}

// Forward all Logger interface methods to the inner logger

func (d *BaseLoggerDecorator) Info(msg string, args ...any) {
	d.inner.Info(msg, args...)
}

func (d *BaseLoggerDecorator) Error(msg string, args ...any) {
	d.inner.Error(msg, args...)
}

func (d *BaseLoggerDecorator) Warn(msg string, args ...any) {
	d.inner.Warn(msg, args...)
}

func (d *BaseLoggerDecorator) Debug(msg string, args ...any) {
	d.inner.Debug(msg, args...)
}

// DualWriterLoggerDecorator logs to two destinations simultaneously.
// This decorator forwards all log calls to both the primary logger and a secondary logger.
type DualWriterLoggerDecorator struct {
	*BaseLoggerDecorator
	secondary Logger
}

// NewDualWriterLoggerDecorator creates a decorator that logs to both primary and secondary loggers.
func NewDualWriterLoggerDecorator(primary, secondary Logger) *DualWriterLoggerDecorator {
	return &DualWriterLoggerDecorator{
		BaseLoggerDecorator: NewBaseLoggerDecorator(primary),
		secondary:           secondary,
	}
}

func (d *DualWriterLoggerDecorator) Info(msg string, args ...any) {
	d.inner.Info(msg, args...)
	d.secondary.Info(msg, args...)
}

func (d *DualWriterLoggerDecorator) Error(msg string, args ...any) {
	d.inner.Error(msg, args...)
	d.secondary.Error(msg, args...)
}

func (d *DualWriterLoggerDecorator) Warn(msg string, args ...any) {
	d.inner.Warn(msg, args...)
	d.secondary.Warn(msg, args...)
}

func (d *DualWriterLoggerDecorator) Debug(msg string, args ...any) {
	d.inner.Debug(msg, args...)
	d.secondary.Debug(msg, args...)
}

// ValueInjectionLoggerDecorator automatically injects key-value pairs into all log events.
// This decorator adds configured key-value pairs to every log call.
type ValueInjectionLoggerDecorator struct {
	*BaseLoggerDecorator
	injectedArgs []any
}

// NewValueInjectionLoggerDecorator creates a decorator that automatically injects values into log events.
func NewValueInjectionLoggerDecorator(inner Logger, injectedArgs ...any) *ValueInjectionLoggerDecorator {
	return &ValueInjectionLoggerDecorator{
		BaseLoggerDecorator: NewBaseLoggerDecorator(inner),
		injectedArgs:        injectedArgs,
	}
}

func (d *ValueInjectionLoggerDecorator) combineArgs(originalArgs []any) []any {
	if len(d.injectedArgs) == 0 {
		return originalArgs
	}
	if len(originalArgs) == 0 {
		return d.injectedArgs
	}
	combined := make([]any, 0, len(d.injectedArgs)+len(originalArgs))
	combined = append(combined, d.injectedArgs...)
	combined = append(combined, originalArgs...)
	return combined
}

func (d *ValueInjectionLoggerDecorator) Info(msg string, args ...any) {
	d.inner.Info(msg, d.combineArgs(args)...)
}

func (d *ValueInjectionLoggerDecorator) Error(msg string, args ...any) {
	d.inner.Error(msg, d.combineArgs(args)...)
}

func (d *ValueInjectionLoggerDecorator) Warn(msg string, args ...any) {
	d.inner.Warn(msg, d.combineArgs(args)...)
}

func (d *ValueInjectionLoggerDecorator) Debug(msg string, args ...any) {
	d.inner.Debug(msg, d.combineArgs(args)...)
}

// FilterLoggerDecorator filters log events based on configurable criteria.
// This decorator can filter by log level, message content, or key-value pairs.
type FilterLoggerDecorator struct {
	*BaseLoggerDecorator
	messageFilters []string          // Substrings to filter on
	keyFilters     map[string]string // Key-value pairs to filter on
	levelFilters   map[string]bool   // Log levels to allow
}

// NewFilterLoggerDecorator creates a decorator that filters log events.
// If levelFilters is nil, all levels (info, error, warn, debug) are allowed by default.
func NewFilterLoggerDecorator(inner Logger, messageFilters []string, keyFilters map[string]string, levelFilters map[string]bool) *FilterLoggerDecorator {
	if levelFilters == nil {
		// Default to allowing all standard log levels
		levelFilters = map[string]bool{
			"info":  true,
			"error": true,
			"warn":  true,
			"debug": true,
		}
	}

	return &FilterLoggerDecorator{
		BaseLoggerDecorator: NewBaseLoggerDecorator(inner),
		messageFilters:      messageFilters,
		keyFilters:          keyFilters,
		levelFilters:        levelFilters,
	}
}

func (d *FilterLoggerDecorator) shouldLog(level, msg string, args ...any) bool {
	// Check level filter
	if allowed, exists := d.levelFilters[level]; exists && !allowed {
		return false
	}

	// Check message filters
	for _, filter := range d.messageFilters {
		if strings.Contains(msg, filter) {
			return false // Block if message contains filter string
		}
	}

	// Check key-value filters
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			if filterValue, exists := d.keyFilters[key]; exists {
				// Convert both values to strings for comparison
				argValue := fmt.Sprintf("%v", args[i+1])
				if argValue == filterValue {
					return false // Block if key-value pair matches filter
				}
			}
		}
	}

	return true
}

func (d *FilterLoggerDecorator) Info(msg string, args ...any) {
	if d.shouldLog("info", msg, args...) {
		d.inner.Info(msg, args...)
	}
}

func (d *FilterLoggerDecorator) Error(msg string, args ...any) {
	if d.shouldLog("error", msg, args...) {
		d.inner.Error(msg, args...)
	}
}

func (d *FilterLoggerDecorator) Warn(msg string, args ...any) {
	if d.shouldLog("warn", msg, args...) {
		d.inner.Warn(msg, args...)
	}
}

func (d *FilterLoggerDecorator) Debug(msg string, args ...any) {
	if d.shouldLog("debug", msg, args...) {
		d.inner.Debug(msg, args...)
	}
}

// LevelModifierLoggerDecorator modifies the log level of events.
// This decorator can promote or demote log levels based on configured rules.
type LevelModifierLoggerDecorator struct {
	*BaseLoggerDecorator
	levelMappings map[string]string // Maps from original level to target level
}

// NewLevelModifierLoggerDecorator creates a decorator that modifies log levels.
func NewLevelModifierLoggerDecorator(inner Logger, levelMappings map[string]string) *LevelModifierLoggerDecorator {
	return &LevelModifierLoggerDecorator{
		BaseLoggerDecorator: NewBaseLoggerDecorator(inner),
		levelMappings:       levelMappings,
	}
}

func (d *LevelModifierLoggerDecorator) logWithLevel(originalLevel, msg string, args ...any) {
	targetLevel := originalLevel
	if mapped, exists := d.levelMappings[originalLevel]; exists {
		targetLevel = mapped
	}

	switch targetLevel {
	case "debug":
		d.inner.Debug(msg, args...)
	case "info":
		d.inner.Info(msg, args...)
	case "warn":
		d.inner.Warn(msg, args...)
	case "error":
		d.inner.Error(msg, args...)
	default:
		// If unknown level, use original
		switch originalLevel {
		case "debug":
			d.inner.Debug(msg, args...)
		case "info":
			d.inner.Info(msg, args...)
		case "warn":
			d.inner.Warn(msg, args...)
		case "error":
			d.inner.Error(msg, args...)
		}
	}
}

func (d *LevelModifierLoggerDecorator) Info(msg string, args ...any) {
	d.logWithLevel("info", msg, args...)
}

func (d *LevelModifierLoggerDecorator) Error(msg string, args ...any) {
	d.logWithLevel("error", msg, args...)
}

func (d *LevelModifierLoggerDecorator) Warn(msg string, args ...any) {
	d.logWithLevel("warn", msg, args...)
}

func (d *LevelModifierLoggerDecorator) Debug(msg string, args ...any) {
	d.logWithLevel("debug", msg, args...)
}

// PrefixLoggerDecorator adds a prefix to all log messages.
// This decorator automatically prepends a configured prefix to every log message.
type PrefixLoggerDecorator struct {
	*BaseLoggerDecorator
	prefix string
}

// NewPrefixLoggerDecorator creates a decorator that adds a prefix to log messages.
func NewPrefixLoggerDecorator(inner Logger, prefix string) *PrefixLoggerDecorator {
	return &PrefixLoggerDecorator{
		BaseLoggerDecorator: NewBaseLoggerDecorator(inner),
		prefix:              prefix,
	}
}

func (d *PrefixLoggerDecorator) formatMessage(msg string) string {
	if d.prefix == "" {
		return msg
	}
	var builder strings.Builder
	builder.Grow(len(d.prefix) + len(msg) + 1) // Pre-allocate capacity for prefix + space + message
	builder.WriteString(d.prefix)
	builder.WriteString(" ")
	builder.WriteString(msg)
	return builder.String()
}

func (d *PrefixLoggerDecorator) Info(msg string, args ...any) {
	d.inner.Info(d.formatMessage(msg), args...)
}

func (d *PrefixLoggerDecorator) Error(msg string, args ...any) {
	d.inner.Error(d.formatMessage(msg), args...)
}

func (d *PrefixLoggerDecorator) Warn(msg string, args ...any) {
	d.inner.Warn(d.formatMessage(msg), args...)
}

func (d *PrefixLoggerDecorator) Debug(msg string, args ...any) {
	d.inner.Debug(d.formatMessage(msg), args...)
}
