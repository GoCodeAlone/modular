package eventlogger

import (
	"errors"
	"fmt"
)

// Error definitions for the eventlogger module
var (
	// Configuration errors
	ErrInvalidLogLevel      = errors.New("invalid log level")
	ErrInvalidFormat        = errors.New("invalid log format")
	ErrInvalidFlushInterval = errors.New("invalid flush interval")
	ErrInvalidOutputType    = errors.New("invalid output target type")
	ErrMissingFileConfig    = errors.New("missing file configuration for file output target")
	ErrMissingFilePath      = errors.New("missing file path for file output target")
	ErrMissingSyslogConfig  = errors.New("missing syslog configuration for syslog output target")
	ErrInvalidSyslogNetwork = errors.New("invalid syslog network type")

	// Runtime errors
	ErrLoggerNotStarted          = errors.New("event logger not started")
	ErrOutputTargetFailed        = errors.New("output target failed")
	ErrEventBufferFull           = errors.New("event buffer is full")
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
	ErrUnknownOutputTargetType   = errors.New("unknown output target type")
	ErrFileNotOpen               = errors.New("file not open")
	ErrSyslogWriterNotInit       = errors.New("syslog writer not initialized")
)

// OutputTargetError wraps errors from output target validation
type OutputTargetError struct {
	Index int
	Err   error
}

func (e *OutputTargetError) Error() string {
	return fmt.Sprintf("output target %d: %v", e.Index, e.Err)
}

func (e *OutputTargetError) Unwrap() error {
	return e.Err
}

// NewOutputTargetError creates a new OutputTargetError
func NewOutputTargetError(index int, err error) *OutputTargetError {
	return &OutputTargetError{
		Index: index,
		Err:   err,
	}
}
