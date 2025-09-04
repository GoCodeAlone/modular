//go:build windows || wasm || js

package eventlogger

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// SyslogTarget stub for unsupported platforms.
type SyslogTarget struct {
	config OutputTargetConfig
	logger modular.Logger
}

// NewSyslogTarget returns an error indicating syslog is unsupported on this platform.
func NewSyslogTarget(config OutputTargetConfig, logger modular.Logger) (*SyslogTarget, error) { //nolint:ireturn
	return nil, fmt.Errorf("syslog output target not supported on this platform")
}

// Start is a no-op.
func (s *SyslogTarget) Start(ctx context.Context) error { return nil }

// Stop is a no-op.
func (s *SyslogTarget) Stop(ctx context.Context) error { return nil }

// WriteEvent is a no-op.
func (s *SyslogTarget) WriteEvent(entry *LogEntry) error { return nil }

// Flush is a no-op.
func (s *SyslogTarget) Flush() error { return nil }
