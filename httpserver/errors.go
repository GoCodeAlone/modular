package httpserver

import (
	"errors"
)

// Error definitions
var (
	// ErrNoSubjectForEventEmission is returned when trying to emit events without a subject
	ErrNoSubjectForEventEmission = errors.New("no subject available for event emission")
)
