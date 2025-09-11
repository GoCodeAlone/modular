//go:build !windows && !wasm && !js

package eventlogger

import (
	"context"
	"fmt"
	"log/syslog"

	"github.com/CrisisTextLine/modular"
)

// SyslogTarget outputs events to syslog (supported on Unix-like systems).
type SyslogTarget struct {
	config OutputTargetConfig
	logger modular.Logger
	writer *syslog.Writer
}

// NewSyslogTarget creates a new syslog output target.
func NewSyslogTarget(config OutputTargetConfig, logger modular.Logger) (*SyslogTarget, error) {
	if config.Syslog == nil {
		return nil, ErrMissingSyslogConfig
	}
	return &SyslogTarget{config: config, logger: logger}, nil
}

// Start initializes the syslog target.
func (s *SyslogTarget) Start(ctx context.Context) error { //nolint:contextcheck
	priority := syslog.LOG_INFO | syslog.LOG_USER
	if f := s.config.Syslog.Facility; f != "" {
		switch f {
		case "kern":
			priority = syslog.LOG_INFO | syslog.LOG_KERN
		case "user":
			priority = syslog.LOG_INFO | syslog.LOG_USER
		case "mail":
			priority = syslog.LOG_INFO | syslog.LOG_MAIL
		case "daemon":
			priority = syslog.LOG_INFO | syslog.LOG_DAEMON
		case "auth":
			priority = syslog.LOG_INFO | syslog.LOG_AUTH
		case "local0":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL0
		case "local1":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL1
		case "local2":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL2
		case "local3":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL3
		case "local4":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL4
		case "local5":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL5
		case "local6":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL6
		case "local7":
			priority = syslog.LOG_INFO | syslog.LOG_LOCAL7
		}
	}
	var err error
	if s.config.Syslog.Network == "unix" {
		s.writer, err = syslog.New(priority, s.config.Syslog.Tag)
	} else {
		s.writer, err = syslog.Dial(s.config.Syslog.Network, s.config.Syslog.Address, priority, s.config.Syslog.Tag)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to syslog: %w", err)
	}
	s.logger.Debug("Syslog output target started", "network", s.config.Syslog.Network, "address", s.config.Syslog.Address)
	return nil
}

// Stop shuts down the syslog target.
func (s *SyslogTarget) Stop(ctx context.Context) error { //nolint:contextcheck
	if s.writer != nil {
		_ = s.writer.Close()
		s.writer = nil
	}
	s.logger.Debug("Syslog output target stopped")
	return nil
}

// WriteEvent writes a log entry to syslog.
func (s *SyslogTarget) WriteEvent(entry *LogEntry) error {
	if s.writer == nil {
		return ErrSyslogWriterNotInit
	}
	if !shouldLogLevel(entry.Level, s.config.Level) {
		return nil
	}
	msg := fmt.Sprintf("[%s] %s: %v", entry.Type, entry.Source, entry.Data)
	switch entry.Level {
	case "DEBUG":
		return s.writer.Debug(msg)
	case "INFO":
		return s.writer.Info(msg)
	case "WARN":
		return s.writer.Warning(msg)
	case "ERROR":
		return s.writer.Err(msg)
	default:
		return s.writer.Info(msg)
	}
}

// Flush flushes syslog output (no-op for syslog).
func (s *SyslogTarget) Flush() error { return nil }
