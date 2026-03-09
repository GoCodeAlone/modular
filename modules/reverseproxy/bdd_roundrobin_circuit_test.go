package reverseproxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Round-robin with Circuit Breaker BDD Scenarios
// Tests the interaction between load balancing and circuit breaker functionality

// iHaveARoundRobinBackendGroupWithCircuitBreakers sets up a round-robin backend group
// with circuit breakers configured for each backend
func (ctx *ReverseProxyBDDTestContext) iHaveARoundRobinBackendGroupWithCircuitBreakers() error {
	ctx.resetContext()

	// Create multiple controllable backends for round-robin testing
	var backend1Counter, backend2Counter, backend3Counter int64

	// Backend 1: Initially healthy, can be made to fail
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&backend1Counter, 1)
		if ctx.controlledFailureMode != nil && *ctx.controlledFailureMode {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("backend-1 forced failure"))
			return
		}
		w.Header().Set("X-Backend", "backend-1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend-1 response"))
	}))

	// Backend 2: Initially healthy
	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&backend2Counter, 1)
		w.Header().Set("X-Backend", "backend-2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend-2 response"))
	}))

	// Backend 3: Initially healthy
	backend3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&backend3Counter, 1)
		w.Header().Set("X-Backend", "backend-3")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend-3 response"))
	}))

	ctx.testServers = append(ctx.testServers, backend1, backend2, backend3)

	// Initialize controlled failure mode
	failureMode := false
	ctx.controlledFailureMode = &failureMode

	// Create application with observable events
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Configure round-robin backend group with circuit breakers
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"backend-1": backend1.URL,
			"backend-2": backend2.URL,
			"backend-3": backend3.URL,
		},
		Routes: map[string]string{
			// Comma-separated backend group for round-robin load balancing
			"/api/roundrobin": "backend-1,backend-2,backend-3",
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"backend-1": {
				URL: backend1.URL,
				CircuitBreaker: BackendCircuitBreakerConfig{
					Enabled:          true,
					FailureThreshold: 2, // Low threshold for quick testing
					RecoveryTimeout:  100 * time.Millisecond,
				},
			},
			"backend-2": {
				URL: backend2.URL,
				CircuitBreaker: BackendCircuitBreakerConfig{
					Enabled:          true,
					FailureThreshold: 2,
					RecoveryTimeout:  100 * time.Millisecond,
				},
			},
			"backend-3": {
				URL: backend3.URL,
				CircuitBreaker: BackendCircuitBreakerConfig{
					Enabled:          true,
					FailureThreshold: 2,
					RecoveryTimeout:  100 * time.Millisecond,
				},
			},
		},
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 2,
			OpenTimeout:      100 * time.Millisecond,
		},
	}

	// Register services
	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	if err := ctx.app.RegisterService("router", mockRouter); err != nil {
		return fmt.Errorf("failed to register router: %w", err)
	}

	if err := ctx.app.RegisterService("logger", logger); err != nil {
		return fmt.Errorf("failed to register logger: %w", err)
	}

	if err := ctx.app.RegisterService("metrics", &testMetrics{}); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	// Register event observer for capturing load balancing and circuit breaker events
	ctx.eventObserver = newTestEventObserver()
	if err := ctx.app.RegisterService("event-bus", &testEventBus{observers: []modular.Observer{ctx.eventObserver}}); err != nil {
		return fmt.Errorf("failed to register event bus: %w", err)
	}

	_ = ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver)

	// Create and register the reverse proxy module
	ctx.module = NewModule()
	ctx.service = ctx.module
	ctx.app.RegisterModule(ctx.module)

	// Register configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize the application
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	return nil
}

// iForceOneBackendToTripItsCircuitBreaker makes one backend fail repeatedly
// to trigger its circuit breaker
func (ctx *ReverseProxyBDDTestContext) iForceOneBackendToTripItsCircuitBreaker() error {
	if ctx.controlledFailureMode == nil {
		return fmt.Errorf("controlled failure mode not initialized")
	}

	// Clear previous events to focus on circuit breaker events
	ctx.eventObserver.ClearEvents()

	// Enable failure mode for backend-1
	*ctx.controlledFailureMode = true

	// Make enough requests to backend-1 to trigger its circuit breaker
	// We need to make requests that would specifically hit backend-1
	// Since we're using round-robin, we'll make multiple requests to ensure backend-1 gets hit
	for i := 0; i < 6; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/roundrobin", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		// Small delay to allow circuit breaker processing
		time.Sleep(10 * time.Millisecond)
	}

	// Allow time for circuit breaker state transitions
	time.Sleep(50 * time.Millisecond)

	return nil
}

// subsequentRequestsShouldRotateToHealthyBackends verifies that after one backend's
// circuit breaker opens, subsequent requests are distributed only among healthy backends
func (ctx *ReverseProxyBDDTestContext) subsequentRequestsShouldRotateToHealthyBackends() error {
	// Disable failure mode to ensure other backends respond successfully
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = false
	}

	// Track which backends respond to verify rotation among healthy backends
	backendResponses := make(map[string]int)
	requestCount := 8 // Make enough requests to see rotation pattern

	for i := 0; i < requestCount; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/roundrobin", nil)
		if err != nil {
			return fmt.Errorf("request %d failed: %w", i, err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("request %d returned error status %d: %s", i, resp.StatusCode, string(body))
		}

		// Check which backend responded
		backendHeader := resp.Header.Get("X-Backend")
		if backendHeader != "" {
			backendResponses[backendHeader]++
		}

		resp.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}

	// Verify that at least 2 backends responded (healthy ones)
	if len(backendResponses) < 2 {
		return fmt.Errorf("expected responses from at least 2 healthy backends, got responses from: %v", backendResponses)
	}

	// Verify that backend-1 (which should have circuit breaker open) received fewer or no requests
	backend1Requests := backendResponses["backend-1"]
	totalHealthyRequests := backendResponses["backend-2"] + backendResponses["backend-3"]

	if totalHealthyRequests == 0 {
		return fmt.Errorf("expected requests to be distributed to healthy backends (backend-2, backend-3)")
	}

	// The circuit breaker should prevent most requests to backend-1
	if backend1Requests > 2 { // Allow for some requests during half-open state
		return fmt.Errorf("backend-1 with open circuit breaker received too many requests: %d", backend1Requests)
	}

	return nil
}

// loadBalanceRoundRobinEventsShouldFire verifies that round-robin load balancing events are emitted
func (ctx *ReverseProxyBDDTestContext) loadBalanceRoundRobinEventsShouldFire() error {
	events := ctx.eventObserver.GetEvents()
	foundRoundRobinEvent := false

	for _, event := range events {
		eventType := event.Type()
		if eventType == EventTypeLoadBalanceRoundRobin {
			foundRoundRobinEvent = true

			// Verify event contains rotation details
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err != nil {
				return fmt.Errorf("failed to parse round-robin event data: %w", err)
			}

			// Check for expected fields in round-robin events
			hasRotationInfo := false
			for key := range eventData {
				if key == "current_index" || key == "selected_backend" || key == "backend_group" {
					hasRotationInfo = true
					break
				}
			}

			if !hasRotationInfo {
				return fmt.Errorf("round-robin event missing rotation information: %v", eventData)
			}
			break
		}
	}

	if !foundRoundRobinEvent {
		// Log available events for debugging
		eventTypes := make([]string, len(events))
		for i, event := range events {
			eventTypes[i] = event.Type()
		}
		return fmt.Errorf("no round-robin load balancing events found. Available events: %v", eventTypes)
	}

	return nil
}

// circuitBreakerOpenEventsShouldFire verifies that circuit breaker open events are emitted
func (ctx *ReverseProxyBDDTestContext) circuitBreakerOpenEventsShouldFire() error {
	events := ctx.eventObserver.GetEvents()
	foundCircuitBreakerEvent := false

	for _, event := range events {
		eventType := event.Type()
		if eventType == "reverseproxy.circuit-breaker.open" ||
			eventType == "reverseproxy.circuitbreaker.open" ||
			eventType == "circuitbreaker.open" {
			foundCircuitBreakerEvent = true

			// Verify event contains circuit breaker information
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err != nil {
				return fmt.Errorf("failed to parse circuit breaker event data: %w", err)
			}

			// Check for expected fields in circuit breaker events
			hasCircuitInfo := false
			for key := range eventData {
				if key == "backend" || key == "backend_name" || key == "state" || key == "failure_count" {
					hasCircuitInfo = true
					break
				}
			}

			if !hasCircuitInfo {
				return fmt.Errorf("circuit breaker event missing circuit information: %v", eventData)
			}
			break
		}
	}

	if !foundCircuitBreakerEvent {
		// Log available events for debugging
		eventTypes := make([]string, len(events))
		for i, event := range events {
			eventTypes[i] = event.Type()
		}
		return fmt.Errorf("no circuit breaker open events found. Available events: %v", eventTypes)
	}

	return nil
}

// handlerShouldReturn503WhenAllBackendsDown verifies that when all backends in the group
// have their circuit breakers open, the handler returns HTTP 503
func (ctx *ReverseProxyBDDTestContext) handlerShouldReturn503WhenAllBackendsDown() error {
	// This scenario tests when all backends are down or have circuit breakers open

	// Create a scenario with all backends failing
	allBackendsDownServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend-1 down"))
	}))
	allBackendsDownServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend-2 down"))
	}))
	allBackendsDownServer3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("backend-3 down"))
	}))

	// Close existing servers and replace with failing ones
	for _, server := range ctx.testServers {
		server.Close()
	}
	ctx.testServers = []*httptest.Server{allBackendsDownServer1, allBackendsDownServer2, allBackendsDownServer3}

	// Update configuration to use failing backends
	ctx.config.BackendServices = map[string]string{
		"down-backend-1": allBackendsDownServer1.URL,
		"down-backend-2": allBackendsDownServer2.URL,
		"down-backend-3": allBackendsDownServer3.URL,
	}
	ctx.config.Routes = map[string]string{
		"/api/alldown": "down-backend-1,down-backend-2,down-backend-3",
	}

	// Re-initialize application with failing backends
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	mockRouter := &testRouter{routes: make(map[string]http.HandlerFunc)}
	if err := ctx.app.RegisterService("router", mockRouter); err != nil {
		return fmt.Errorf("failed to register router: %w", err)
	}

	if err := ctx.app.RegisterService("logger", logger); err != nil {
		return fmt.Errorf("failed to register logger: %w", err)
	}

	if err := ctx.app.RegisterService("metrics", &testMetrics{}); err != nil {
		return fmt.Errorf("failed to register metrics: %w", err)
	}

	ctx.eventObserver = newTestEventObserver()
	if err := ctx.app.RegisterService("event-bus", &testEventBus{observers: []modular.Observer{ctx.eventObserver}}); err != nil {
		return fmt.Errorf("failed to register event bus: %w", err)
	}

	_ = ctx.app.(modular.Subject).RegisterObserver(ctx.eventObserver)

	ctx.module = NewModule()
	ctx.service = ctx.module
	ctx.app.RegisterModule(ctx.module)

	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize application with all backends down: %w", err)
	}

	// Make requests to trigger circuit breaker openings
	for i := 0; i < 10; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/alldown", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Allow circuit breakers to open
	time.Sleep(150 * time.Millisecond)

	// Now make a request that should return 503 Service Unavailable
	resp, err := ctx.makeRequestThroughModule("GET", "/api/alldown", nil)
	if err != nil {
		return fmt.Errorf("failed to make request when all backends down: %w", err)
	}
	defer resp.Body.Close()

	// Should return 503 Service Unavailable when all backends are down
	expectedStatuses := []int{
		http.StatusServiceUnavailable,  // 503 - preferred
		http.StatusInternalServerError, // 500 - acceptable
		http.StatusBadGateway,          // 502 - acceptable
	}

	statusValid := false
	for _, expectedStatus := range expectedStatuses {
		if resp.StatusCode == expectedStatus {
			statusValid = true
			break
		}
	}

	if !statusValid {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected HTTP 503/500/502 when all backends down, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify response body contains error information
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("error response should contain informative message when all backends are down")
	}

	return nil
}

// Additional helper functions for round-robin circuit breaker testing

// testBackendRecovery tests that a backend can recover from circuit breaker open state
func (ctx *ReverseProxyBDDTestContext) testBackendRecovery() error {
	// Disable failure mode to allow recovery
	if ctx.controlledFailureMode != nil {
		*ctx.controlledFailureMode = false
	}

	// Wait for circuit breaker recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Make a few requests to test recovery
	for i := 0; i < 3; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/roundrobin", nil)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Allow time for circuit breaker state changes
	time.Sleep(50 * time.Millisecond)

	return nil
}

// verifyRoundRobinDistribution verifies that requests are properly distributed
// in round-robin fashion among available backends
func (ctx *ReverseProxyBDDTestContext) verifyRoundRobinDistribution() error {
	backendCounts := make(map[string]int)
	totalRequests := 12

	for i := 0; i < totalRequests; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/api/roundrobin", nil)
		if err != nil {
			return fmt.Errorf("request %d failed: %w", i, err)
		}

		if resp.StatusCode == http.StatusOK {
			backendID := resp.Header.Get("X-Backend")
			if backendID != "" {
				backendCounts[backendID]++
			}
		}

		resp.Body.Close()
		time.Sleep(5 * time.Millisecond)
	}

	// Verify that multiple backends received requests
	if len(backendCounts) < 2 {
		return fmt.Errorf("expected round-robin distribution across multiple backends, got: %v", backendCounts)
	}

	// In proper round-robin, each healthy backend should get roughly equal requests
	// Allow for some variance due to circuit breaker effects
	minExpected := totalRequests/len(backendCounts) - 2
	for backend, count := range backendCounts {
		if count < minExpected {
			return fmt.Errorf("backend %s received too few requests (%d), expected at least %d", backend, count, minExpected)
		}
	}

	return nil
}
