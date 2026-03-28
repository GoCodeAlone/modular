package scheduler

import (
	"errors"
)

// Module-specific errors for scheduler module.
// These errors are defined locally to ensure proper linting compliance.
var (
	// ErrNoSubjectForEventEmission is returned when trying to emit events without a subject
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
)
