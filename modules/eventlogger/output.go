package eventlogger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoCodeAlone/modular"
)

// OutputTarget defines the interface for event log output targets.
type OutputTarget interface {
	// Start initializes the output target
	Start(ctx context.Context) error

	// Stop shuts down the output target
	Stop(ctx context.Context) error

	// WriteEvent writes a log entry to the output target
	WriteEvent(entry *LogEntry) error

	// Flush ensures all buffered events are written
	Flush() error
}

// NewOutputTarget creates a new output target based on configuration.
func NewOutputTarget(config OutputTargetConfig, logger modular.Logger) (OutputTarget, error) {
	switch config.Type {
	case "console":
		return NewConsoleTarget(config, logger), nil
	case "file":
		return NewFileTarget(config, logger)
	case "syslog":
		return NewSyslogTarget(config, logger)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownOutputTargetType, config.Type)
	}
}

// ConsoleTarget outputs events to console/stdout.
type ConsoleTarget struct {
	config OutputTargetConfig
	logger modular.Logger
	writer io.Writer
}

// NewConsoleTarget creates a new console output target.
func NewConsoleTarget(config OutputTargetConfig, logger modular.Logger) *ConsoleTarget {
	return &ConsoleTarget{
		config: config,
		logger: logger,
		writer: os.Stdout,
	}
}

// Start initializes the console target.
func (c *ConsoleTarget) Start(ctx context.Context) error {
	c.logger.Debug("Console output target started")
	return nil
}

// Stop shuts down the console target.
func (c *ConsoleTarget) Stop(ctx context.Context) error {
	c.logger.Debug("Console output target stopped")
	return nil
}

// WriteEvent writes a log entry to console.
func (c *ConsoleTarget) WriteEvent(entry *LogEntry) error {
	// Check log level
	if !shouldLogLevel(entry.Level, c.config.Level) {
		return nil
	}

	var output string
	var err error

	switch c.config.Format {
	case "json":
		output, err = c.formatJSON(entry)
	case "text":
		output, err = c.formatText(entry)
	case "structured":
		output, err = c.formatStructured(entry)
	default:
		output, err = c.formatStructured(entry)
	}

	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	_, err = fmt.Fprintln(c.writer, output)
	if err != nil {
		return fmt.Errorf("failed to write to console: %w", err)
	}
	return nil
}

// Flush flushes console output (no-op for console).
func (c *ConsoleTarget) Flush() error {
	return nil
}

// formatJSON formats a log entry as JSON.
func (c *ConsoleTarget) formatJSON(entry *LogEntry) (string, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entry to JSON: %w", err)
	}
	return string(data), nil
}

// formatText formats a log entry as human-readable text.
func (c *ConsoleTarget) formatText(entry *LogEntry) (string, error) {
	timestamp := ""
	if c.config.Console != nil && c.config.Console.Timestamps {
		timestamp = entry.Timestamp.Format("2006-01-02 15:04:05") + " "
	}

	// Color coding if enabled
	levelStr := entry.Level
	if c.config.Console != nil && c.config.Console.UseColor {
		levelStr = c.colorizeLevel(entry.Level)
	}

	// Format data as string
	dataStr := ""
	if entry.Data != nil {
		dataStr = fmt.Sprintf(" %v", entry.Data)
	}

	return fmt.Sprintf("%s%s [%s] %s%s", timestamp, levelStr, entry.Type, entry.Source, dataStr), nil
}

// formatStructured formats a log entry in structured format.
func (c *ConsoleTarget) formatStructured(entry *LogEntry) (string, error) {
	var builder strings.Builder

	// Timestamp and level
	timestamp := ""
	if c.config.Console != nil && c.config.Console.Timestamps {
		timestamp = entry.Timestamp.Format("2006-01-02 15:04:05")
	}

	levelStr := entry.Level
	if c.config.Console != nil && c.config.Console.UseColor {
		levelStr = c.colorizeLevel(entry.Level)
	}

	if timestamp != "" {
		fmt.Fprintf(&builder, "[%s] %s %s\n", timestamp, levelStr, entry.Type)
	} else {
		fmt.Fprintf(&builder, "%s %s\n", levelStr, entry.Type)
	}

	// Source
	fmt.Fprintf(&builder, "  Source: %s\n", entry.Source)

	// Data
	if entry.Data != nil {
		fmt.Fprintf(&builder, "  Data: %v\n", entry.Data)
	}

	// Metadata
	if len(entry.Metadata) > 0 {
		fmt.Fprintf(&builder, "  Metadata:\n")
		for k, v := range entry.Metadata {
			fmt.Fprintf(&builder, "    %s: %v\n", k, v)
		}
	}

	return strings.TrimSuffix(builder.String(), "\n"), nil
}

// colorizeLevel adds ANSI color codes to log levels.
func (c *ConsoleTarget) colorizeLevel(level string) string {
	switch level {
	case "DEBUG":
		return "\033[36mDEBUG\033[0m" // Cyan
	case "INFO":
		return "\033[32mINFO\033[0m" // Green
	case "WARN":
		return "\033[33mWARN\033[0m" // Yellow
	case "ERROR":
		return "\033[31mERROR\033[0m" // Red
	default:
		return level
	}
}

// FileTarget outputs events to a file with rotation support.
type FileTarget struct {
	config OutputTargetConfig
	logger modular.Logger
	file   *os.File
}

// NewFileTarget creates a new file output target.
func NewFileTarget(config OutputTargetConfig, logger modular.Logger) (*FileTarget, error) {
	if config.File == nil {
		return nil, ErrMissingFileConfig
	}

	target := &FileTarget{
		config: config,
		logger: logger,
	}

	// Proactively ensure the log file path exists so tests can detect it quickly
	if err := os.MkdirAll(filepath.Dir(config.File.Path), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", filepath.Dir(config.File.Path), err)
	}
	// Create the file if it doesn't exist yet (will be reopened on Start)
	f, err := os.OpenFile(config.File.Path, os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		_ = f.Close()
	}

	return target, nil
}

// Start initializes the file target.
func (f *FileTarget) Start(ctx context.Context) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(f.config.File.Path), 0o755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", filepath.Dir(f.config.File.Path), err)
	}
	file, err := os.OpenFile(f.config.File.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", f.config.File.Path, err)
	}
	f.file = file
	f.logger.Debug("File output target started", "path", f.config.File.Path)

	// Force sync so tests can detect the file immediately
	if err := f.file.Sync(); err != nil {
		// Not fatal, but log via logger
		f.logger.Debug("Initial file sync failed", "error", err)
	}
	return nil
}

// Stop shuts down the file target.
func (f *FileTarget) Stop(ctx context.Context) error {
	if f.file != nil {
		f.file.Close()
		f.file = nil
	}
	f.logger.Debug("File output target stopped")
	return nil
}

// WriteEvent writes a log entry to file.
func (f *FileTarget) WriteEvent(entry *LogEntry) error {
	if f.file == nil {
		return ErrFileNotOpen
	}

	// Check log level
	if !shouldLogLevel(entry.Level, f.config.Level) {
		return nil
	}

	var output string
	var err error

	switch f.config.Format {
	case "json":
		output, err = f.formatJSON(entry)
	case "text":
		output, err = f.formatText(entry)
	case "structured":
		output, err = f.formatStructured(entry)
	default:
		output, err = f.formatJSON(entry) // Default to JSON for files
	}

	if err != nil {
		return fmt.Errorf("failed to format log entry: %w", err)
	}

	_, err = fmt.Fprintln(f.file, output)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}

// Flush flushes file output.
func (f *FileTarget) Flush() error {
	if f.file != nil {
		if err := f.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync file: %w", err)
		}
	}
	return nil
}

// formatJSON formats a log entry as JSON for file output.
func (f *FileTarget) formatJSON(entry *LogEntry) (string, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entry to JSON: %w", err)
	}
	return string(data), nil
}

// formatText formats a log entry as text for file output.
func (f *FileTarget) formatText(entry *LogEntry) (string, error) {
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	dataStr := ""
	if entry.Data != nil {
		dataStr = fmt.Sprintf(" %v", entry.Data)
	}
	return fmt.Sprintf("%s %s [%s] %s%s", timestamp, entry.Level, entry.Type, entry.Source, dataStr), nil
}

// formatStructured formats a log entry in structured format for file output.
func (f *FileTarget) formatStructured(entry *LogEntry) (string, error) {
	var builder strings.Builder

	// Timestamp and level
	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	fmt.Fprintf(&builder, "[%s] %s %s | Source: %s", timestamp, entry.Level, entry.Type, entry.Source)

	// Data
	if entry.Data != nil {
		fmt.Fprintf(&builder, " | Data: %v", entry.Data)
	}

	// Metadata
	if len(entry.Metadata) > 0 {
		fmt.Fprintf(&builder, " | Metadata: %v", entry.Metadata)
	}

	return builder.String(), nil
}

// Syslog target implementation moved to platform-specific files (syslog_output_unix.go & syslog_output_stub.go)

// shouldLogLevel checks if a log level should be included based on minimum level.
func shouldLogLevel(eventLevel, minLevel string) bool {
	levels := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}

	eventLevelNum, ok1 := levels[eventLevel]
	minLevelNum, ok2 := levels[minLevel]

	if !ok1 || !ok2 {
		return true // Default to logging if levels are invalid
	}

	return eventLevelNum >= minLevelNum
}
