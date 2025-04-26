package reverseproxy

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCircuitBreaker(t *testing.T) {
	// Test constructor function
	cb := NewCircuitBreaker(5, 10*time.Second)

	assert.Equal(t, CircuitClosed, cb.GetState(), "New circuit breaker should start in closed state")
	assert.Equal(t, 0, cb.failureCount, "New circuit breaker should have zero failures")
	assert.Equal(t, 5, cb.failureThreshold, "Failure threshold should be set correctly")
	assert.Equal(t, 10*time.Second, cb.resetTimeout, "Reset timeout should be set correctly")
}

func TestCircuitBreakerRecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker(5, 10*time.Second)

	// Record some failures but not enough to open circuit
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Record a success, which should reset the counter
	cb.RecordSuccess()
	assert.Equal(t, 0, cb.failureCount, "Success should reset failure counter")
	assert.Equal(t, CircuitClosed, cb.GetState(), "Circuit should remain closed")

	// Test transition from half-open to closed
	cb.state = CircuitHalfOpen
	cb.RecordSuccess()
	assert.Equal(t, CircuitClosed, cb.GetState(), "Success in half-open state should close the circuit")
}

func TestCircuitBreakerRecordFailure(t *testing.T) {
	cb := NewCircuitBreaker(5, 10*time.Second)

	// Record failures up to threshold
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
		assert.Equal(t, CircuitClosed, cb.GetState(), "Circuit should remain closed before threshold")
		assert.Equal(t, i+1, cb.failureCount, "Failure count should be incremented")
	}

	// One more failure should trip the circuit
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState(), "Circuit should open after threshold failures")
	assert.Equal(t, 5, cb.failureCount, "Failure count should be at threshold")

	// Further failures don't change state
	cb.RecordFailure()
	assert.Equal(t, CircuitOpen, cb.GetState(), "Circuit should remain open on additional failures")
	assert.Equal(t, 5, cb.failureCount, "Failure count should remain at threshold")
}

func TestCircuitBreakerIsOpen(t *testing.T) {
	cb := NewCircuitBreaker(5, 10*time.Millisecond) // Short timeout for testing

	// When closed
	assert.False(t, cb.IsOpen(), "New circuit should not be open")

	// Trip the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// Should be open now
	assert.True(t, cb.IsOpen(), "Circuit should be open after failures")

	// Wait for timeout to expire
	time.Sleep(20 * time.Millisecond)

	// First call after timeout should transition to half-open and return false
	assert.False(t, cb.IsOpen(), "Circuit should transition to half-open after timeout")
	assert.Equal(t, CircuitHalfOpen, cb.GetState(), "State should be half-open")

	// Test half-open state
	assert.False(t, cb.IsOpen(), "Half-open circuit should report as not open")
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(5, 10*time.Second)

	// Trip the circuit
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	assert.Equal(t, CircuitOpen, cb.GetState(), "Circuit should be open")

	// Reset the circuit
	cb.Reset()

	assert.Equal(t, CircuitClosed, cb.GetState(), "Circuit should be closed after reset")
	assert.Equal(t, 0, cb.failureCount, "Failure count should be reset")
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(100, 10*time.Second)

	// Test concurrent access to the circuit breaker
	done := make(chan bool)

	// Start 10 goroutines to record failures
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				cb.RecordFailure()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, CircuitOpen, cb.GetState(), "Circuit should be open after concurrent failures")

	// Reset and test concurrent successes
	cb.Reset()

	// Trip the circuit part way
	for i := 0; i < 50; i++ {
		cb.RecordFailure()
	}

	// Start 10 goroutines to record successes
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				cb.RecordSuccess()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, CircuitClosed, cb.GetState(), "Circuit should be closed after concurrent successes")
	assert.Equal(t, 0, cb.failureCount, "Failure count should be reset")
}

func TestCircuitBreakerConfiguration(t *testing.T) {
	// Create a composite handler with a test backend
	backends := []*Backend{
		{
			ID:     "api1",
			URL:    "http://example.com/api1",
			Client: http.DefaultClient,
		},
		{
			ID:     "api2",
			URL:    "http://example.com/api2",
			Client: http.DefaultClient,
		},
	}

	handler := NewCompositeHandler(backends, true, 5*time.Second)

	// Initially, circuit breakers should be nil
	for _, b := range backends {
		assert.Nil(t, handler.circuitBreakers[b.ID], "Circuit breaker should be nil initially")
	}

	// Setup configuration
	globalConfig := CircuitBreakerConfig{
		Enabled:             true,
		FailureThreshold:    10,
		ResetTimeoutSeconds: 60,
	}

	// Backend-specific configuration for api2
	backendConfig := map[string]CircuitBreakerConfig{
		"api2": {
			Enabled:             true,
			FailureThreshold:    3,
			ResetTimeoutSeconds: 15,
		},
	}

	// Apply the configuration
	handler.ConfigureCircuitBreakers(globalConfig, backendConfig)

	// Check that circuit breakers are configured correctly
	assert.NotNil(t, handler.circuitBreakers["api1"], "Circuit breaker for api1 should be created")
	assert.NotNil(t, handler.circuitBreakers["api2"], "Circuit breaker for api2 should be created")

	// Check that global config was used for api1
	assert.Equal(t, CircuitClosed, handler.circuitBreakers["api1"].GetState(), "Circuit breaker should start closed")
	assert.Equal(t, 10, handler.circuitBreakers["api1"].failureThreshold, "Global failure threshold should be used")
	assert.Equal(t, 60*time.Second, handler.circuitBreakers["api1"].resetTimeout, "Global reset timeout should be used")

	// Check that specific config was used for api2
	assert.Equal(t, CircuitClosed, handler.circuitBreakers["api2"].GetState(), "Circuit breaker should start closed")
	assert.Equal(t, 3, handler.circuitBreakers["api2"].failureThreshold, "Backend-specific failure threshold should be used")
	assert.Equal(t, 15*time.Second, handler.circuitBreakers["api2"].resetTimeout, "Backend-specific reset timeout should be used")

	// Now test the disabled case
	disabledConfig := map[string]CircuitBreakerConfig{
		"api1": {
			Enabled: false,
		},
	}

	// Reset and apply new configuration
	handler = NewCompositeHandler(backends, true, 5*time.Second)
	handler.ConfigureCircuitBreakers(globalConfig, disabledConfig)

	// api1 should be disabled, api2 should use global config
	assert.Nil(t, handler.circuitBreakers["api1"], "Circuit breaker for api1 should be disabled")
	assert.NotNil(t, handler.circuitBreakers["api2"], "Circuit breaker for api2 should be created")
}

func TestGlobalCircuitBreakerDisabled(t *testing.T) {
	// Create a composite handler with a test backend
	backends := []*Backend{
		{
			ID:     "api1",
			URL:    "http://example.com/api1",
			Client: http.DefaultClient,
		},
	}

	handler := NewCompositeHandler(backends, true, 5*time.Second)

	// Create configuration with circuit breaker disabled globally
	globalConfig := CircuitBreakerConfig{
		Enabled: false,
	}

	// Apply the configuration
	handler.ConfigureCircuitBreakers(globalConfig, nil)

	// Circuit breakers should be nil since they're disabled globally
	assert.Nil(t, handler.circuitBreakers["api1"], "Circuit breaker should be disabled")
}
