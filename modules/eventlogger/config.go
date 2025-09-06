package eventlogger

import (
	"time"
)

// EventLoggerConfig holds configuration for the event logger module.
type EventLoggerConfig struct {
	// Enabled determines if event logging is active
	Enabled bool `yaml:"enabled" default:"true" desc:"Enable event logging"`

	// LogLevel determines which events to log (DEBUG, INFO, WARN, ERROR)
	LogLevel string `yaml:"logLevel" default:"INFO" desc:"Minimum log level for events"`

	// Format specifies the output format (text, json, structured)
	Format string `yaml:"format" default:"structured" desc:"Log output format"`

	// OutputTargets specifies where to output logs
	OutputTargets []OutputTargetConfig `yaml:"outputTargets" desc:"Output targets for event logs"`

	// EventTypeFilters allows filtering which event types to log
	EventTypeFilters []string `yaml:"eventTypeFilters" desc:"Event types to log (empty = all events)"`

	// BufferSize sets the size of the event buffer for async processing
	BufferSize int `yaml:"bufferSize" default:"100" desc:"Buffer size for async event processing"`

	// FlushInterval sets how often to flush buffered events
	FlushInterval time.Duration `yaml:"flushInterval" default:"5s" desc:"Interval to flush buffered events"`

	// IncludeMetadata determines if event metadata should be logged
	IncludeMetadata bool `yaml:"includeMetadata" default:"true" desc:"Include event metadata in logs"`

	// IncludeStackTrace determines if stack traces should be logged for error events
	IncludeStackTrace bool `yaml:"includeStackTrace" default:"false" desc:"Include stack traces for error events"`

	// StartupSync forces startup operational events (config loaded, outputs registered, logger started)
	// to be emitted synchronously during Start() instead of via async goroutine+sleep.
	StartupSync bool `yaml:"startupSync" default:"false" desc:"Emit startup operational events synchronously (no artificial sleep)"`

	// ShutdownEmitStopped controls whether a logger.stopped operational event is emitted.
	// When false, the module will not emit com.modular.eventlogger.stopped to avoid races with shutdown.
	ShutdownEmitStopped bool `yaml:"shutdownEmitStopped" default:"true" desc:"Emit logger stopped operational event on Stop"`

	// ShutdownDrainTimeout specifies how long Stop() should wait for in-flight events to drain.
	// Zero or negative duration means "wait indefinitely" (Stop blocks until all events processed).
	// This allows operators to explicitly choose between a bounded shutdown and a fully
	// lossless drain. A very large positive value is NOT treated specially—only <=0 triggers
	// the indefinite behavior.
	ShutdownDrainTimeout time.Duration `yaml:"shutdownDrainTimeout" default:"2s" desc:"Maximum time to wait for draining event queue on Stop. Zero or negative = unlimited wait."`
}

// OutputTargetConfig configures a specific output target for event logs.
type OutputTargetConfig struct {
	// Type specifies the output type (console, file, syslog)
	Type string `yaml:"type" default:"console" desc:"Output target type"`

	// Level allows different log levels per target
	Level string `yaml:"level" default:"INFO" desc:"Minimum log level for this target"`

	// Format allows different formats per target
	Format string `yaml:"format" default:"structured" desc:"Log format for this target"`

	// Configuration specific to the target type
	Console *ConsoleTargetConfig `yaml:"console,omitempty" desc:"Console output configuration"`
	File    *FileTargetConfig    `yaml:"file,omitempty" desc:"File output configuration"`
	Syslog  *SyslogTargetConfig  `yaml:"syslog,omitempty" desc:"Syslog output configuration"`
}

// ConsoleTargetConfig configures console output.
type ConsoleTargetConfig struct {
	// UseColor enables colored output for console
	UseColor bool `yaml:"useColor" default:"true" desc:"Enable colored console output"`

	// Timestamps determines if timestamps should be included
	Timestamps bool `yaml:"timestamps" default:"true" desc:"Include timestamps in console output"`
}

// FileTargetConfig configures file output.
type FileTargetConfig struct {
	// Path specifies the log file path
	Path string `yaml:"path" required:"true" desc:"Path to log file"`

	// MaxSize specifies the maximum file size in MB before rotation
	MaxSize int `yaml:"maxSize" default:"100" desc:"Maximum file size in MB before rotation"`

	// MaxBackups specifies the maximum number of backup files to keep
	MaxBackups int `yaml:"maxBackups" default:"5" desc:"Maximum number of backup files"`

	// MaxAge specifies the maximum age in days to keep log files
	MaxAge int `yaml:"maxAge" default:"30" desc:"Maximum age in days to keep log files"`

	// Compress determines if rotated logs should be compressed
	Compress bool `yaml:"compress" default:"true" desc:"Compress rotated log files"`
}

// SyslogTargetConfig configures syslog output.
type SyslogTargetConfig struct {
	// Network specifies the network type (tcp, udp, unix)
	Network string `yaml:"network" default:"unix" desc:"Network type for syslog connection"`

	// Address specifies the syslog server address
	Address string `yaml:"address" default:"" desc:"Syslog server address"`

	// Tag specifies the syslog tag
	Tag string `yaml:"tag" default:"modular" desc:"Syslog tag"`

	// Facility specifies the syslog facility
	Facility string `yaml:"facility" default:"user" desc:"Syslog facility"`
}

// Validate implements the ConfigValidator interface for EventLoggerConfig.
func (c *EventLoggerConfig) Validate() error {
	// Validate log level
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true,
	}
	if !validLevels[c.LogLevel] {
		return ErrInvalidLogLevel
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true, "json": true, "structured": true,
	}
	if !validFormats[c.Format] {
		return ErrInvalidFormat
	}

	// Validate flush interval
	if c.FlushInterval <= 0 {
		return ErrInvalidFlushInterval
	}

	// Validate output targets
	for i, target := range c.OutputTargets {
		if err := target.Validate(); err != nil {
			return NewOutputTargetError(i, err)
		}
	}

	return nil
}

// Validate validates an OutputTargetConfig.
func (o *OutputTargetConfig) Validate() error {
	// Validate type
	validTypes := map[string]bool{
		"console": true, "file": true, "syslog": true,
	}
	if !validTypes[o.Type] {
		return ErrInvalidOutputType
	}

	// Validate level
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true,
	}
	if !validLevels[o.Level] {
		return ErrInvalidLogLevel
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true, "json": true, "structured": true,
	}
	if !validFormats[o.Format] {
		return ErrInvalidFormat
	}

	// Type-specific validation
	switch o.Type {
	case "file":
		if o.File == nil {
			return ErrMissingFileConfig
		}
		if o.File.Path == "" {
			return ErrMissingFilePath
		}
	case "syslog":
		if o.Syslog == nil {
			return ErrMissingSyslogConfig
		}
		validNetworks := map[string]bool{
			"tcp": true, "udp": true, "unix": true,
		}
		if !validNetworks[o.Syslog.Network] {
			return ErrInvalidSyslogNetwork
		}
	}

	return nil
}
