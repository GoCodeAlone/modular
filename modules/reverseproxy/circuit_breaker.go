// Package reverseproxy provides circuit breaker implementation for backend proxies.
package reverseproxy

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// StateClosed indicates the circuit is closed and allowing requests.
	StateClosed CircuitState = iota
	// StateOpen indicates the circuit is open and blocking requests.
	StateOpen
	// StateHalfOpen indicates the circuit is allowing a test request.
	StateHalfOpen
)

// String returns a string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen is returned when the circuit is open and requests are not allowed.
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern for HTTP requests.
type CircuitBreaker struct {
	failureThreshold int           // Number of failures before opening circuit
	resetTimeout     time.Duration // How long to wait before trying again
	requestTimeout   time.Duration // Timeout for requests
	failureCount     int           // Current count of consecutive failures
	lastFailure      time.Time     // When the last failure occurred
	state            CircuitState  // Current state of the circuit
	mutex            sync.RWMutex
	metricsCollector *MetricsCollector
	backendName      string
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failureCount = 0
	cb.state = StateClosed
}

// Reset resets the circuit breaker to closed state (capitalized for backward compatibility).
func (cb *CircuitBreaker) Reset() {
	cb.reset()
}

// NewCircuitBreaker creates a new CircuitBreaker with default settings.
func NewCircuitBreaker(backendName string, metricsCollector *MetricsCollector) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: 5,
		resetTimeout:     10 * time.Second,
		requestTimeout:   5 * time.Second,
		state:            StateClosed, // Start closed
		metricsCollector: metricsCollector,
		backendName:      backendName,
	}
}

// NewCircuitBreakerWithConfig creates a new CircuitBreaker with custom settings.
func NewCircuitBreakerWithConfig(backendName string, config CircuitBreakerConfig, metricsCollector *MetricsCollector) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: config.FailureThreshold,
		resetTimeout:     config.OpenTimeout,             // Using OpenTimeout from config.go
		requestTimeout:   time.Duration(5) * time.Second, // Default request timeout
		state:            StateClosed,                    // Start closed
		metricsCollector: metricsCollector,
		backendName:      backendName,
	}
}

// IsOpen returns true if the circuit is open (no requests should be made).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	if cb.state == StateOpen {
		// Circuit is open, check if reset timeout has passed
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			// Allow a single request to check if the service has recovered
			cb.state = StateHalfOpen
			return false
		}
		return true
	}

	return false
}

// RecordSuccess records a successful request and resets the failure count.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// If circuit was open or half-open, close it
	if cb.state != StateClosed {
		cb.state = StateClosed
		if cb.metricsCollector != nil {
			cb.metricsCollector.SetCircuitBreakerStatus(cb.backendName, false)
		}
	}

	cb.failureCount = 0
}

// RecordFailure records a failed request and opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Only increment failure count if we haven't reached the threshold
	if cb.failureCount < cb.failureThreshold {
		cb.failureCount++
	}

	cb.lastFailure = time.Now()

	// Open the circuit if failure threshold reached
	if cb.failureCount >= cb.failureThreshold && cb.state == StateClosed {
		cb.state = StateOpen
		if cb.metricsCollector != nil {
			cb.metricsCollector.SetCircuitBreakerStatus(cb.backendName, true)
		}
	}
}

// Execute executes the provided function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(req *http.Request, fn func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if cb.IsOpen() {
		return nil, ErrCircuitOpen
	}

	// Start timing the request
	startTime := time.Now()

	// Save the current state for metrics reporting
	initialState := cb.GetState()

	// Create a context with timeout
	ctx := req.Context()
	if cb.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cb.requestTimeout)
		defer cancel()
	}

	// Execute with timeout context
	req = req.WithContext(ctx)
	resp, err := fn(req)

	// Record metrics
	var statusCode int
	if resp != nil {
		statusCode = resp.StatusCode
	}

	if cb.metricsCollector != nil {
		cb.metricsCollector.RecordRequest(
			cb.backendName,
			startTime,
			statusCode,
			err,
			map[string]string{"circuit_state": initialState.String()},
		)
	}

	// Handle the result
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		cb.RecordFailure()
		return resp, err
	}

	cb.RecordSuccess()
	return resp, err
}

// GetState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count of the circuit breaker.
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount
}

// WithFailureThreshold sets the number of failures required to open the circuit.
func (cb *CircuitBreaker) WithFailureThreshold(threshold int) *CircuitBreaker {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.failureThreshold = threshold
	return cb
}

// WithResetTimeout sets the duration to wait before allowing a test request.
func (cb *CircuitBreaker) WithResetTimeout(timeout time.Duration) *CircuitBreaker {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.resetTimeout = timeout
	return cb
}

// WithRequestTimeout sets the timeout for each request protected by this circuit breaker.
func (cb *CircuitBreaker) WithRequestTimeout(timeout time.Duration) *CircuitBreaker {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.requestTimeout = timeout
	return cb
}

// WithMetricsCollector associates a metrics collector with this circuit breaker.
func (cb *CircuitBreaker) WithMetricsCollector(collector *MetricsCollector) *CircuitBreaker {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.metricsCollector = collector
	return cb
}
