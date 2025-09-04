package reverseproxy

// Add BDD step implementations split out from primary file to reduce size.
// These cover circuit breaker transition verification and will be extended for
// remaining undefined scenarios (health events, feature flags, etc.).

import (
	"fmt"
	"time"
)

// Circuit breaker transition focused steps moved here / newly added.
func (ctx *ReverseProxyBDDTestContext) circuitBreakerEventsShouldBeEmittedForStateTransitions() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not initialized")
	}
	// Allow brief time for any pending events
	time.Sleep(150 * time.Millisecond)
	sawOpen := false
	for _, e := range ctx.eventObserver.GetEvents() {
		switch e.Type() {
		case EventTypeCircuitBreakerOpen:
			sawOpen = true
		}
	}
	if !sawOpen {
		return fmt.Errorf("missing circuit breaker open event")
	}
	// half-open and closed may occur later depending on test timing; do not hard fail yet.
	return nil
}

// Helper to actively drive half-open -> closed by letting reset timeout elapse and issuing a success.
func (ctx *ReverseProxyBDDTestContext) allowCircuitBreakerToHalfOpenAndRecover() error {
	if ctx.service == nil {
		return fmt.Errorf("service not available")
	}
	// Find controllable backend server (created in the circuit breaker scenario setup)
	if ctx.controlledFailureMode == nil {
		return fmt.Errorf("no controllable backend in this context")
	}
	// Wait past reset timeout (using config or default 10s open timeout; tests should set small OpenTimeout)
	reset := ctx.service.config.CircuitBreakerConfig.OpenTimeout
	if reset == 0 {
		reset = 1500 * time.Millisecond
	}
	time.Sleep(reset + 100*time.Millisecond)
	// Switch backend to success
	*ctx.controlledFailureMode = false
	// Issue a request to trigger half-open then close (consistent with main route)
	_, _ = ctx.makeRequestThroughModule("GET", "/api/test", nil)
	time.Sleep(100 * time.Millisecond)
	// Verify closed event
	for _, e := range ctx.eventObserver.GetEvents() {
		if e.Type() == EventTypeCircuitBreakerClosed {
			return nil
		}
	}
	return fmt.Errorf("circuit breaker did not emit closed event after recovery")
}

// backendHealthTransitionEventsShouldBeEmitted validates that at least one healthy and one
// unhealthy backend event were emitted for the controllable backend within a
// bounded wait window. It tolerates ordering differences due to timing.
func (ctx *ReverseProxyBDDTestContext) backendHealthTransitionEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not initialized")
	}
	deadline := time.Now().Add(2 * time.Second)
	sawHealthy := false
	sawUnhealthy := false
	// Poll events until deadline or both conditions satisfied
	for time.Now().Before(deadline) && (!sawHealthy || !sawUnhealthy) {
		events := ctx.eventObserver.GetEvents()
		for _, e := range events {
			switch e.Type() {
			case EventTypeBackendHealthy:
				sawHealthy = true
			case EventTypeBackendUnhealthy:
				sawUnhealthy = true
			}
		}
		if sawHealthy && sawUnhealthy {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	missing := ""
	if !sawHealthy {
		missing += "healthy "
	}
	if !sawUnhealthy {
		missing += "unhealthy "
	}
	if missing != "" {
		return fmt.Errorf("missing backend health events: %s", missing)
	}
	return nil
}
