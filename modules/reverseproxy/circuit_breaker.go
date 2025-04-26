package reverseproxy

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitClosed means the circuit is closed and requests flow normally
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit is open and requests fail immediately
	CircuitOpen
	// CircuitHalfOpen means the circuit is testing if the backend has recovered
	CircuitHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures
// when a backend service is unreliable or failing
type CircuitBreaker struct {
	state            CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	mutex            sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the specified failure threshold and reset timeout
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureCount:     0,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

// RecordSuccess records a successful request and resets the failure counter
// If the circuit was half-open, it closes the circuit
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failureCount = 0

	// If the circuit was half-open and we had a success, close it
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
	}
}

// RecordFailure records a failed request and potentially opens the circuit
// if the failure threshold is exceeded
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.lastFailureTime = time.Now()

	// If the circuit is already open, nothing to do
	if cb.state == CircuitOpen {
		return
	}

	// Increment failure count
	cb.failureCount++

	// If we've exceeded the threshold, open the circuit
	if cb.failureCount >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// IsOpen returns true if the circuit is open and requests should not be processed
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// If circuit is open, check if it's time to try again
	if cb.state == CircuitOpen {
		// If reset timeout has elapsed, transition to half-open
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			// Must release the read lock to acquire write lock
			cb.mutex.RUnlock()
			cb.mutex.Lock()

			// Double-check after acquiring write lock
			if cb.state == CircuitOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
				cb.state = CircuitHalfOpen
			}

			// Downgrade write lock to read lock
			stateToReturn := cb.state
			cb.mutex.Unlock()
			cb.mutex.RLock()

			// Return false if we transitioned to half-open
			return stateToReturn == CircuitOpen
		}
		return true
	}

	return false
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = CircuitClosed
	cb.failureCount = 0
}
