package reverseproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Missing implementation for setting up backends with health checking enabled
func (ctx *ReverseProxyBDDTestContext) iHaveBackendsWithHealthCheckingEnabled() error {
	ctx.resetContext()

	// Create backend servers with controllable health status
	healthyBackendHealthy := true
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthyBackendHealthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy backend response"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("unhealthy backend response"))
		}
	}))
	ctx.testServers = append(ctx.testServers, healthyServer)

	// Create unhealthy backend server
	unhealthyBackendHealthy := false
	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unhealthyBackendHealthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("now healthy backend response"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("unhealthy backend response"))
		}
	}))
	ctx.testServers = append(ctx.testServers, unhealthyServer)

	// Configure reverse proxy with health checking enabled
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"healthy-backend":   healthyServer.URL,
			"unhealthy-backend": unhealthyServer.URL,
		},
		Routes: map[string]string{
			"/api/healthy":   "healthy-backend",
			"/api/unhealthy": "unhealthy-backend",
		},
		DefaultBackend: "healthy-backend",
		HealthCheck: HealthCheckConfig{
			Enabled:                true,
			Interval:               500 * time.Millisecond, // Frequent checks for testing
			Timeout:                200 * time.Millisecond,
			RecentRequestThreshold: 1 * time.Second,
			HealthEndpoints: map[string]string{
				"healthy-backend":   "/health",
				"unhealthy-backend": "/health",
			},
			ExpectedStatusCodes: []int{200},
		},
		BackendConfigs: map[string]BackendServiceConfig{
			"healthy-backend": {
				URL: healthyServer.URL,
			},
			"unhealthy-backend": {
				URL: unhealthyServer.URL,
			},
		},
	}

	// Store references to control backend health status
	ctx.healthyBackendHealthy = &healthyBackendHealthy
	ctx.unhealthyBackendHealthy = &unhealthyBackendHealthy

	// Setup application with event observation enabled FIRST
	if err := ctx.setupApplicationWithConfig(); err != nil {
		return err
	}

	// CRITICAL: Register observer AFTER application is set up but BEFORE health checker starts
	if ctx.app != nil {
		// Create observer if it doesn't exist
		if ctx.eventObserver == nil {
			ctx.eventObserver = newTestEventObserver()
		}
		// Always register observer with the current app (it may have been recreated)
		if obsApp, ok := ctx.app.(modular.Subject); ok {
			if err := obsApp.RegisterObserver(ctx.eventObserver); err != nil {
				return fmt.Errorf("failed to register event observer for health events: %w", err)
			}
		}
	}

	// Force health checker to use our event emitter
	if ctx.module != nil && ctx.module.healthChecker != nil {
		// Ensure health checker event emitter is properly connected
		ctx.module.healthChecker.SetEventEmitter(func(eventType string, data map[string]interface{}) {
			// Direct emission to our module's emitEvent method which connects to observers
			ctx.module.emitEvent(context.Background(), eventType, data)
		})
	}

	return nil
}
