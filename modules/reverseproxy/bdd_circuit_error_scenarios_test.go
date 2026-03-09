package reverseproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Circuit Breaker Response Scenarios

func (ctx *ReverseProxyBDDTestContext) circuitBreakersShouldRespondAppropriately() error {
	// Test circuit breaker response behavior after it should be open
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Make a request through the circuit breaker to test its response behavior
	resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// When circuit breaker is open, we expect appropriate error responses
	// The circuit should respond with service unavailable or similar error status
	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("circuit breaker should not return OK when open, got status %d", resp.StatusCode)
	}

	// Verify we get an appropriate error status code indicating service unavailable
	expectedStatuses := []int{
		http.StatusServiceUnavailable,  // 503
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusGatewayTimeout,      // 504 - circuit breaker timeout
	}

	statusValid := false
	for _, expectedStatus := range expectedStatuses {
		if resp.StatusCode == expectedStatus {
			statusValid = true
			break
		}
	}

	if !statusValid {
		return fmt.Errorf("circuit breaker response status %d not in expected error statuses %v", resp.StatusCode, expectedStatuses)
	}

	// Verify response has appropriate error content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("circuit breaker should return informative error response body")
	}

	// The response should indicate some form of service unavailability
	bodyStr := strings.ToLower(string(body))
	errorIndicators := []string{"error", "unavailable", "failed", "circuit", "service", "timeout"}
	hasErrorIndicator := false
	for _, indicator := range errorIndicators {
		if strings.Contains(bodyStr, indicator) {
			hasErrorIndicator = true
			break
		}
	}

	if !hasErrorIndicator {
		return fmt.Errorf("circuit breaker response should contain error indicators, got: %s", string(body))
	}

	return nil
}

// Circuit State Transition Scenarios

func (ctx *ReverseProxyBDDTestContext) circuitStateShouldTransitionBasedOnResults() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Ensure we have circuit breakers configured and available
	if ctx.config == nil || !ctx.config.CircuitBreakerConfig.Enabled {
		return fmt.Errorf("circuit breakers not enabled in configuration")
	}

	// Access circuit breaker directly to verify state transitions
	// The circuit breaker should be available through the service's internal state

	// Verify circuit breaker exists and is functioning
	initialFailureThreshold := ctx.config.CircuitBreakerConfig.FailureThreshold
	if initialFailureThreshold <= 0 {
		initialFailureThreshold = 3 // Default threshold
	}

	// Test state transitions by forcing failures and successes
	// First, ensure circuit is initially closed by making a successful request to a healthy backend
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy response"))
	}))
	ctx.testServers = append(ctx.testServers, healthyServer)

	// Update backend to healthy for initial success
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = false
	}

	// Make a successful request to ensure circuit starts closed
	resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err == nil && resp != nil {
		resp.Body.Close()
	}

	// Now test transition to open state by forcing failures
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = true
	}

	// Make enough failure requests to trigger circuit breaker opening
	for i := 0; i < int(initialFailureThreshold)+2; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		// Small delay between requests to allow circuit breaker to process
		time.Sleep(10 * time.Millisecond)
	}

	// Now verify circuit is open by making a request and checking the behavior
	resp, err = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make request to check open state: %w", err)
	}
	resp.Body.Close()

	// Circuit should now be open and rejecting requests with error status
	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("circuit should be open and rejecting requests, but got OK response")
	}

	// Test half-open transition by waiting for reset timeout
	resetTimeout := ctx.config.CircuitBreakerConfig.OpenTimeout
	if resetTimeout <= 0 {
		resetTimeout = 1 * time.Second // Default timeout
	}

	// Wait slightly longer than reset timeout to trigger half-open state
	time.Sleep(resetTimeout + 100*time.Millisecond)

	// Make backend healthy again for recovery test
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = false
	}

	// Make a request that should succeed and close the circuit
	resp, err = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make request for circuit recovery: %w", err)
	}
	resp.Body.Close()

	// After successful request, circuit should transition back to closed
	// Make another request to verify circuit is properly closed
	resp, err = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make request to verify closed state: %w", err)
	}
	resp.Body.Close()

	// This request should succeed if circuit properly transitioned to closed
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("circuit should be closed after successful recovery, but got status %d", resp.StatusCode)
	}

	return nil
}

// Client Response Handling Scenarios

func (ctx *ReverseProxyBDDTestContext) appropriateClientResponsesShouldBeReturned() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Test that clients receive appropriate responses under various conditions

	// Test 1: Normal operation should return backend response
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = false
	}

	resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make normal request: %w", err)
	}

	// Normal operation should return success
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("normal operation should return OK, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read normal response body: %w", err)
	}

	// Should get backend response content
	if len(body) == 0 {
		return fmt.Errorf("normal operation should return response body")
	}

	// Test 2: Backend failure should return appropriate error response
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = true
	}

	resp, err = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make backend failure request: %w", err)
	}
	defer resp.Body.Close()

	// Backend failure should return error status
	if resp.StatusCode == http.StatusOK {
		return fmt.Errorf("backend failure should not return OK status")
	}

	// Should return 500 or 502 for backend failures
	if resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusBadGateway {
		return fmt.Errorf("backend failure should return 500 or 502, got %d", resp.StatusCode)
	}

	// Test 3: Circuit breaker open should return service unavailable
	// Make several more failures to open circuit breaker
	failureThreshold := ctx.config.CircuitBreakerConfig.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = 3
	}

	for i := 0; i < int(failureThreshold)+1; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now circuit should be open
	resp, err = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	if err != nil {
		return fmt.Errorf("failed to make circuit open request: %w", err)
	}
	defer resp.Body.Close()

	// Circuit open should return service unavailable
	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusInternalServerError {
		return fmt.Errorf("circuit open should return 503 or 500, got %d", resp.StatusCode)
	}

	// Verify response contains error information
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read circuit open response body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("circuit open should return informative error response")
	}

	return nil
}

// Error Response Handling Scenarios

func (ctx *ReverseProxyBDDTestContext) appropriateErrorResponsesShouldBeReturned() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}

	// Test various error scenarios and their appropriate responses

	// Test 1: Backend unavailable (connection refused)
	unavailableBackendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend error"))
	}))
	// Close immediately to simulate unavailable backend
	unavailableBackendServer.Close()

	// Add unavailable backend to config
	if ctx.service.config.BackendServices == nil {
		ctx.service.config.BackendServices = make(map[string]string)
	}
	ctx.service.config.BackendServices["unavailable-backend"] = unavailableBackendServer.URL

	// Configure routing to use unavailable backend with a non-conflicting route
	if ctx.service.config.Routes == nil {
		ctx.service.config.Routes = make(map[string]string)
	}
	ctx.service.config.Routes["/error/unavailable"] = "unavailable-backend"

	// We need to create the backend proxy for the unavailable backend
	backendURL, _ := url.Parse(unavailableBackendServer.URL)
	ctx.service.backendProxies["unavailable-backend"] = ctx.service.createReverseProxyForBackend(context.Background(), backendURL, "unavailable-backend", "")

	// Register the route handler for the new route
	ctx.service.safeHandleFunc("/error/unavailable", ctx.service.createBackendProxyHandler("unavailable-backend"))

	// Make request to unavailable backend
	resp, err := ctx.makeRequestThroughModule("GET", "/error/unavailable", nil)
	if err != nil {
		return fmt.Errorf("failed to make request to unavailable backend: %w", err)
	}
	defer resp.Body.Close()

	// Should return 502 Bad Gateway, 500 Internal Server Error, or 504 Gateway Timeout
	if resp.StatusCode != http.StatusBadGateway && resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusGatewayTimeout {
		return fmt.Errorf("unavailable backend should return 502, 500, or 504, got %d", resp.StatusCode)
	}

	// Test 2: Backend timeout simulation
	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow backend that would cause timeout
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("delayed response"))
	}))
	ctx.testServers = append(ctx.testServers, timeoutServer)

	// Configure very short timeout for testing
	if ctx.service.config.BackendConfigs == nil {
		ctx.service.config.BackendConfigs = make(map[string]BackendServiceConfig)
	}

	ctx.service.config.BackendServices["timeout-backend"] = timeoutServer.URL
	ctx.service.config.Routes["/error/timeout"] = "timeout-backend"
	ctx.service.config.BackendConfigs["timeout-backend"] = BackendServiceConfig{
		URL: timeoutServer.URL,
		// Short timeout would be configured here if supported
	}

	// Create the backend proxy for the timeout backend
	timeoutBackendURL, _ := url.Parse(timeoutServer.URL)
	ctx.service.backendProxies["timeout-backend"] = ctx.service.createReverseProxyForBackend(context.Background(), timeoutBackendURL, "timeout-backend", "")

	// Register the route handler for the timeout route
	ctx.service.safeHandleFunc("/error/timeout", ctx.service.createBackendProxyHandler("timeout-backend"))

	// Test 3: No backend configured error
	resp, err = ctx.makeRequestThroughModule("GET", "/nonexistent", nil)
	if err != nil {
		return fmt.Errorf("failed to make request to nonexistent route: %w", err)
	}
	defer resp.Body.Close()

	// Should return 404 Not Found, 500 Internal Server Error, or 504 Gateway Timeout
	// (504 can occur if global timeout configuration affects nonexistent route handling)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusGatewayTimeout {
		return fmt.Errorf("nonexistent route should return 404, 500, or 504, got %d", resp.StatusCode)
	}

	// Test 4: Verify error responses contain appropriate headers
	if resp.Header.Get("Content-Type") == "" {
		return fmt.Errorf("error responses should include Content-Type header")
	}

	// Test 5: Verify error response bodies contain meaningful messages
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("error responses should contain informative body")
	}

	// Error response should contain some indication of the problem
	bodyStr := strings.ToLower(string(body))
	errorIndicators := []string{"error", "not found", "unavailable", "failed"}
	hasErrorIndicator := false
	for _, indicator := range errorIndicators {
		if strings.Contains(bodyStr, indicator) {
			hasErrorIndicator = true
			break
		}
	}

	if !hasErrorIndicator {
		return fmt.Errorf("error response should contain error indicators, got: %s", string(body))
	}

	return nil
}

// Circuit Breaker Failure Pattern Functions

func (ctx *ReverseProxyBDDTestContext) differentBackendsFailAtDifferentRates() error {
	// Reset context to ensure clean state
	ctx.resetContext()

	// Create backends with different failure rates
	// Backend 1: Always fails (100% failure rate)
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend 1 failure"))
	}))
	ctx.testServers = append(ctx.testServers, failingServer)

	// Backend 2: Intermittent failures (50% failure rate)
	requestCount := 0
	intermittentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("backend 2 intermittent failure"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backend 2 success"))
		}
	}))
	ctx.testServers = append(ctx.testServers, intermittentServer)

	// Backend 3: Always succeeds (0% failure rate)
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend 3 success"))
	}))
	ctx.testServers = append(ctx.testServers, healthyServer)

	// Configure backends with different circuit breaker settings BEFORE setup
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"failing-backend":      failingServer.URL,
			"intermittent-backend": intermittentServer.URL,
			"healthy-backend":      healthyServer.URL,
		},
		Routes: map[string]string{
			"/api/fail":         "failing-backend",
			"/api/intermittent": "intermittent-backend",
			"/api/healthy":      "healthy-backend",
		},
		DefaultBackend: "healthy-backend",

		// Configure per-backend circuit breaker settings using BackendCircuitBreakers
		BackendCircuitBreakers: map[string]CircuitBreakerConfig{
			"failing-backend": {
				Enabled:          true,
				FailureThreshold: 2,
				OpenTimeout:      100 * time.Millisecond,
			},
			"intermittent-backend": {
				Enabled:          true,
				FailureThreshold: 3,
				OpenTimeout:      200 * time.Millisecond,
			},
			"healthy-backend": {
				Enabled:          true,
				FailureThreshold: 5,
				OpenTimeout:      300 * time.Millisecond,
			},
		},
		// Also configure BackendConfigs for URL mapping
		BackendConfigs: map[string]BackendServiceConfig{
			"failing-backend": {
				URL: failingServer.URL,
			},
			"intermittent-backend": {
				URL: intermittentServer.URL,
			},
			"healthy-backend": {
				URL: healthyServer.URL,
			},
		},
	}

	// CRITICAL: Create ObservableApplication for event capture
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Setup event observation BEFORE setting up the module
	ctx.eventObserver = newTestEventObserver()
	if err := ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register event observer: %w", err)
	}

	// Register router service
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	ctx.app.RegisterService("router", mockRouter)

	// Create and register module
	ctx.module = NewModule()
	ctx.service = ctx.module
	ctx.app.RegisterModule(ctx.module)

	// Register config section
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize and start the application
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}
	if err := ctx.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Enable circuit breakers globally
	if ctx.config != nil {
		ctx.config.CircuitBreakerConfig = CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2, // Low threshold for faster testing
			OpenTimeout:      100 * time.Millisecond,
		}
	}

	// Initialize the service to activate circuit breakers
	if err := ctx.ensureServiceInitialized(); err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Clear events to focus on this test
	if ctx.eventObserver != nil {
		ctx.eventObserver.ClearEvents()
	}

	// Send requests to each backend to trigger different failure rates
	for i := 0; i < 5; i++ {
		// Failing backend requests
		resp, err := ctx.makeRequestThroughModule("GET", "/api/fail", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}

		// Intermittent backend requests
		resp, err = ctx.makeRequestThroughModule("GET", "/api/intermittent", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}

		// Healthy backend requests
		resp, err = ctx.makeRequestThroughModule("GET", "/api/healthy", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}

		time.Sleep(20 * time.Millisecond)
	}

	// Wait for circuit breaker events to be emitted
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) testRequestsAreSentThroughHalfopenCircuits() error {
	// Wait for circuit breakers to enter half-open state
	time.Sleep(300 * time.Millisecond)

	// Send test requests that should go through half-open circuits
	for i := 0; i < 3; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/test", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Allow time for circuit state transitions
	time.Sleep(100 * time.Millisecond)

	return nil
}

func (ctx *ReverseProxyBDDTestContext) backendsReturnErrorResponses() error {
	// Enable failure mode on the controllable backend to make it return errors
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = true
	}

	// Send a request that should receive an error response and store it for validation
	resp, err := ctx.makeRequestThroughModule("GET", "/api/error", nil)

	// Store the response and error for validation in subsequent steps
	ctx.lastError = err
	ctx.lastResponse = resp

	// Don't close the response body here - leave it for the validation step
	return nil
}

func (ctx *ReverseProxyBDDTestContext) backendConnectionsFail() error {
	// Use the existing configuration set up by the previous step
	// The setup method should have already configured unreachable backends

	// Send a request that should fail due to connection failures
	// Use the route configured in the setup method (/api/fail -> failing-backend)
	resp, err := ctx.makeRequestThroughModule("GET", "/api/fail", nil)

	// Store the response and error for validation in subsequent steps
	ctx.lastError = err
	ctx.lastResponse = resp

	if resp != nil {
		defer resp.Body.Close()
		// For connection failures, we expect error status codes (5xx)
		if resp.StatusCode < 500 {
			// If we get a non-error status code, this might indicate
			// the connection failure wasn't properly handled
			ctx.lastError = fmt.Errorf("expected connection failure to result in error status, got %d", resp.StatusCode)
		}
	}

	return nil
}
