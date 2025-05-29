// Package reverseproxy provides error definitions for the reverse proxy module.
package reverseproxy

import "errors"

// Error definitions for the reverse proxy module.
var (
	// ErrCircuitOpen defined in circuit_breaker.go
	ErrMaxRetriesReached  = errors.New("maximum number of retries reached")
	ErrRequestTimeout     = errors.New("request timed out")
	ErrNoAvailableBackend = errors.New("no available backend")
)
