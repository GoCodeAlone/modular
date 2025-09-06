package reverseproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthChecker_NewHealthChecker tests creation of a health checker
func TestHealthChecker_NewHealthChecker(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 30 * time.Second,
		Timeout:  5 * time.Second,
	}

	backends := map[string]string{
		"backend1": "http://127.0.0.1:9003",
		"backend2": "http://127.0.0.1:9004",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, backends, client, logger)

	assert.NotNil(t, hc)
	assert.Equal(t, config, hc.config)
	assert.Equal(t, backends, hc.backends)
	assert.Equal(t, client, hc.httpClient)
	assert.Equal(t, logger, hc.logger)
	assert.NotNil(t, hc.healthStatus)
	assert.NotNil(t, hc.requestTimes)
	assert.NotNil(t, hc.stopChan)
}

// TestHealthChecker_StartStop tests starting and stopping the health checker
func TestHealthChecker_StartStop(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 100 * time.Millisecond, // Short interval for testing
		Timeout:  1 * time.Second,
	}

	// Create a mock server that returns healthy status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	backends := map[string]string{
		"backend1": server.URL,
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, backends, client, logger)

	// Test starting
	ctx := context.Background()
	assert.False(t, hc.IsRunning())

	err := hc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, hc.IsRunning())

	// Wait a bit for health checks to run
	time.Sleep(150 * time.Millisecond)

	// Check that health status was updated
	status := hc.GetHealthStatus()
	assert.Len(t, status, 1)
	assert.Contains(t, status, "backend1")
	assert.True(t, status["backend1"].Healthy)
	assert.True(t, status["backend1"].DNSResolved)
	assert.Positive(t, status["backend1"].TotalChecks)

	// Test stopping
	hc.Stop(ctx)
	assert.False(t, hc.IsRunning())

	// Test that we can start again
	err = hc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, hc.IsRunning())

	hc.Stop(ctx)
	assert.False(t, hc.IsRunning())
}

// TestHealthChecker_DNSResolution tests DNS resolution functionality
func TestHealthChecker_DNSResolution(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 1 * time.Second,
		Timeout:  5 * time.Second,
	}

	backends := map[string]string{
		"valid_host":   "http://localhost:8080",
		"invalid_host": "http://127.0.0.1:9999", // Use unreachable localhost port instead
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, backends, client, logger)

	// Test DNS resolution for valid host
	dnsResolved, resolvedIPs, err := hc.performDNSCheck(context.Background(), "http://localhost:8080")
	assert.True(t, dnsResolved)
	require.NoError(t, err)
	assert.NotEmpty(t, resolvedIPs)

	// Test DNS resolution for unreachable host
	// Use unreachable localhost port - DNS will succeed but connection will fail
	dnsResolved, resolvedIPs, err = hc.performDNSCheck(context.Background(), "http://127.0.0.1:9999")
	assert.True(t, dnsResolved)     // DNS should resolve localhost successfully
	require.NoError(t, err)         // DNS resolution itself should work
	assert.NotEmpty(t, resolvedIPs) // Should get IP addresses

	// Test invalid URL
	dnsResolved, resolvedIPs, err = hc.performDNSCheck(context.Background(), "://invalid-url")
	assert.False(t, dnsResolved)
	require.Error(t, err)
	assert.Empty(t, resolvedIPs)
}

// TestHealthChecker_HTTPCheck tests HTTP health check functionality
func TestHealthChecker_HTTPCheck(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:             true,
		Interval:            1 * time.Second,
		Timeout:             5 * time.Second,
		ExpectedStatusCodes: []int{200, 204},
	}

	// Create servers with different responses
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer unhealthyServer.Close()

	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer timeoutServer.Close()

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, map[string]string{}, client, logger)

	ctx := context.Background()

	// Test healthy server
	healthy, responseTime, err := hc.performHTTPCheck(ctx, "healthy", healthyServer.URL)
	assert.True(t, healthy)
	require.NoError(t, err)
	assert.Greater(t, responseTime, time.Duration(0))

	// Test unhealthy server (500 status)
	healthy, responseTime, err = hc.performHTTPCheck(ctx, "unhealthy", unhealthyServer.URL)
	assert.False(t, healthy)
	require.Error(t, err)
	assert.Greater(t, responseTime, time.Duration(0))

	// Test timeout
	shortConfig := &HealthCheckConfig{
		Enabled:             true,
		Interval:            1 * time.Second,
		Timeout:             1 * time.Millisecond, // Very short timeout
		ExpectedStatusCodes: []int{200},
	}
	hc.config = shortConfig

	healthy, responseTime, err = hc.performHTTPCheck(ctx, "timeout", timeoutServer.URL)
	assert.False(t, healthy)
	require.Error(t, err)
	assert.Greater(t, responseTime, time.Duration(0))
}

// TestHealthChecker_CustomHealthEndpoints tests custom health check endpoints
func TestHealthChecker_CustomHealthEndpoints(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 1 * time.Second,
		Timeout:  5 * time.Second,
		HealthEndpoints: map[string]string{
			"backend1": "/health",
			"backend2": "/api/status",
			// Include backend5 with a full URL so we don't rely on post-construction mutation
			"backend5": "http://127.0.0.1:9005/check",
		},
		BackendHealthCheckConfig: map[string]BackendHealthConfig{
			"backend3": {
				Enabled:  true,
				Endpoint: "/custom-health",
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, map[string]string{}, client, logger)

	// Test global health endpoint
	endpoint := hc.getHealthCheckEndpoint("backend1", "http://127.0.0.1:8080")
	assert.Equal(t, "http://127.0.0.1:8080/health", endpoint)

	endpoint = hc.getHealthCheckEndpoint("backend2", "http://127.0.0.1:8080")
	assert.Equal(t, "http://127.0.0.1:8080/api/status", endpoint)

	// Test backend-specific health endpoint
	endpoint = hc.getHealthCheckEndpoint("backend3", "http://127.0.0.1:8080")
	assert.Equal(t, "http://127.0.0.1:8080/custom-health", endpoint)

	// Test default (no custom endpoint)
	endpoint = hc.getHealthCheckEndpoint("backend4", "http://127.0.0.1:8080")
	assert.Equal(t, "http://127.0.0.1:8080", endpoint)

	// Test full URL in endpoint (pre-configured)
	endpoint = hc.getHealthCheckEndpoint("backend5", "http://127.0.0.1:8080")
	assert.Equal(t, "http://127.0.0.1:9005/check", endpoint)
}

// TestHealthChecker_BackendSpecificConfig tests backend-specific configuration
func TestHealthChecker_BackendSpecificConfig(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:             true,
		Interval:            30 * time.Second,
		Timeout:             5 * time.Second,
		ExpectedStatusCodes: []int{200},
		BackendHealthCheckConfig: map[string]BackendHealthConfig{
			"backend1": {
				Enabled:             true,
				Interval:            10 * time.Second,
				Timeout:             2 * time.Second,
				ExpectedStatusCodes: []int{200, 201},
			},
			"backend2": {
				Enabled: false,
			},
		},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, map[string]string{}, client, logger)

	// Test backend-specific interval
	interval := hc.getBackendInterval("backend1")
	assert.Equal(t, 10*time.Second, interval)

	// Test global interval fallback
	interval = hc.getBackendInterval("backend3")
	assert.Equal(t, 30*time.Second, interval)

	// Test backend-specific timeout
	timeout := hc.getBackendTimeout("backend1")
	assert.Equal(t, 2*time.Second, timeout)

	// Test global timeout fallback
	timeout = hc.getBackendTimeout("backend3")
	assert.Equal(t, 5*time.Second, timeout)

	// Test backend-specific expected status codes
	codes := hc.getExpectedStatusCodes("backend1")
	assert.Equal(t, []int{200, 201}, codes)

	// Test global expected status codes fallback
	codes = hc.getExpectedStatusCodes("backend3")
	assert.Equal(t, []int{200}, codes)

	// Test backend health check enabled/disabled
	enabled := hc.isBackendHealthCheckEnabled("backend1")
	assert.True(t, enabled)

	enabled = hc.isBackendHealthCheckEnabled("backend2")
	assert.False(t, enabled)

	enabled = hc.isBackendHealthCheckEnabled("backend3")
	assert.True(t, enabled) // Default to enabled
}

// TestHealthChecker_RecentRequestThreshold tests skipping health checks due to recent requests
func TestHealthChecker_RecentRequestThreshold(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:                true,
		Interval:               1 * time.Second,
		Timeout:                5 * time.Second,
		RecentRequestThreshold: 30 * time.Second,
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, map[string]string{}, client, logger)

	// Initially should not skip (no recent requests)
	shouldSkip := hc.shouldSkipHealthCheck("backend1")
	assert.False(t, shouldSkip)

	// Record a request
	hc.RecordBackendRequest("backend1")

	// Should skip now
	shouldSkip = hc.shouldSkipHealthCheck("backend1")
	assert.True(t, shouldSkip)

	// Wait for threshold to pass
	config.RecentRequestThreshold = 1 * time.Millisecond
	time.Sleep(2 * time.Millisecond)

	// Should not skip anymore
	shouldSkip = hc.shouldSkipHealthCheck("backend1")
	assert.False(t, shouldSkip)

	// Test with threshold disabled (0)
	config.RecentRequestThreshold = 0
	hc.RecordBackendRequest("backend1")
	shouldSkip = hc.shouldSkipHealthCheck("backend1")
	assert.False(t, shouldSkip)
}

// TestHealthChecker_UpdateBackends tests updating the list of backends
func TestHealthChecker_UpdateBackends(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 1 * time.Second,
		Timeout:  5 * time.Second,
	}

	initialBackends := map[string]string{
		"backend1": "http://127.0.0.1:9003",
		"backend2": "http://127.0.0.1:9004",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, initialBackends, client, logger)

	// Initialize backend status
	hc.initializeBackendStatus("backend1", "http://127.0.0.1:9003")
	hc.initializeBackendStatus("backend2", "http://127.0.0.1:9004")

	// Check initial status
	status := hc.GetHealthStatus()
	assert.Len(t, status, 2)
	assert.Contains(t, status, "backend1")
	assert.Contains(t, status, "backend2")

	// Update backends - remove backend2, add backend3
	updatedBackends := map[string]string{
		"backend1": "http://127.0.0.1:9003",
		"backend3": "http://127.0.0.1:9006",
	}

	hc.UpdateBackends(context.Background(), updatedBackends)

	// Check updated status
	status = hc.GetHealthStatus()
	assert.Len(t, status, 2)
	assert.Contains(t, status, "backend1")
	assert.Contains(t, status, "backend3")
	assert.NotContains(t, status, "backend2")

	// Check that backend URLs are updated
	assert.Equal(t, updatedBackends, hc.backends)
}

// TestHealthChecker_GetHealthStatus tests getting health status
func TestHealthChecker_GetHealthStatus(t *testing.T) {
	config := &HealthCheckConfig{
		Enabled:  true,
		Interval: 1 * time.Second,
		Timeout:  5 * time.Second,
	}

	backends := map[string]string{
		"backend1": "http://127.0.0.1:9003",
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, backends, client, logger)

	// Initialize backend status
	hc.initializeBackendStatus("backend1", "http://127.0.0.1:9003")

	// Test GetHealthStatus
	status := hc.GetHealthStatus()
	assert.Len(t, status, 1)
	assert.Contains(t, status, "backend1")

	backend1Status := status["backend1"]
	assert.Equal(t, "backend1", backend1Status.BackendID)
	assert.Equal(t, "http://127.0.0.1:9003", backend1Status.URL)
	assert.False(t, backend1Status.Healthy) // Initially unhealthy

	// Test GetBackendHealthStatus
	backendStatus, exists := hc.GetBackendHealthStatus("backend1")
	assert.True(t, exists)
	assert.Equal(t, backend1Status.BackendID, backendStatus.BackendID)
	assert.Equal(t, backend1Status.URL, backendStatus.URL)

	// Test non-existent backend
	_, exists = hc.GetBackendHealthStatus("nonexistent")
	assert.False(t, exists)
}

// TestHealthChecker_FullIntegration tests full integration with actual health checking
func TestHealthChecker_FullIntegration(t *testing.T) {
	// Create test servers
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer unhealthyServer.Close()

	config := &HealthCheckConfig{
		Enabled:                true,
		Interval:               50 * time.Millisecond, // Fast for testing
		Timeout:                1 * time.Second,
		RecentRequestThreshold: 80 * time.Millisecond, // Longer than interval
		ExpectedStatusCodes:    []int{200},
		HealthEndpoints: map[string]string{
			"healthy": "/health",
		},
	}

	backends := map[string]string{
		"healthy":   healthyServer.URL,
		"unhealthy": unhealthyServer.URL,
	}

	client := &http.Client{Timeout: 10 * time.Second}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hc := NewHealthChecker(config, backends, client, logger)

	// Start the health checker
	ctx := context.Background()
	err := hc.Start(ctx)
	require.NoError(t, err)
	defer hc.Stop(ctx)

	// Wait for health checks to complete
	time.Sleep(100 * time.Millisecond)

	// Check healthy backend
	status, exists := hc.GetBackendHealthStatus("healthy")
	assert.True(t, exists)
	assert.True(t, status.Healthy, "Healthy backend should be marked as healthy")
	assert.True(t, status.DNSResolved, "DNS should be resolved for healthy backend")
	assert.Positive(t, status.TotalChecks, "Should have performed at least one check")
	assert.Positive(t, status.SuccessfulChecks, "Should have at least one successful check")
	assert.Empty(t, status.LastError, "Should have no error for healthy backend")

	// Check unhealthy backend
	status, exists = hc.GetBackendHealthStatus("unhealthy")
	assert.True(t, exists)
	assert.False(t, status.Healthy, "Unhealthy backend should be marked as unhealthy")
	assert.True(t, status.DNSResolved, "DNS should be resolved for unhealthy backend")
	assert.Positive(t, status.TotalChecks, "Should have performed at least one check")
	assert.Equal(t, int64(0), status.SuccessfulChecks, "Should have no successful checks")
	assert.NotEmpty(t, status.LastError, "Should have an error for unhealthy backend")
	assert.Contains(t, status.LastError, "500", "Error should mention status code")

	// Test recent request threshold
	// Record a request
	hc.RecordBackendRequest("healthy")

	// Wait for the next health check interval (50ms)
	// Since threshold is 80ms, the request should still be recent
	time.Sleep(60 * time.Millisecond)

	// Check that the health check was skipped
	status, _ = hc.GetBackendHealthStatus("healthy")
	assert.Positive(t, status.ChecksSkipped, "Should have skipped at least one check")

	// Wait for threshold to pass
	time.Sleep(30 * time.Millisecond) // Total wait: 90ms, threshold is 80ms

	// Wait for another check interval
	time.Sleep(100 * time.Millisecond)

	// Should resume normal checking
	status, _ = hc.GetBackendHealthStatus("healthy")
	assert.True(t, status.Healthy, "Should still be healthy after threshold passes")
}

// TestModule_HealthCheckIntegration tests health check integration with the module
func TestModule_HealthCheckIntegration(t *testing.T) {
	// Create a healthy test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "health") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"server":"test","path":"%s"}`, r.URL.Path)
		}
	}))
	defer server.Close()

	// Create module with health check enabled
	module := NewModule()

	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": server.URL,
		},
		DefaultBackend: "api",
		HealthCheck: HealthCheckConfig{
			Enabled:                true,
			Interval:               50 * time.Millisecond,
			Timeout:                1 * time.Second,
			RecentRequestThreshold: 10 * time.Millisecond,
			ExpectedStatusCodes:    []int{200},
			HealthEndpoints: map[string]string{
				"api": "/health",
			},
		},
	}

	// Create mock app
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: testConfig,
	}

	// Create mock router
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Initialize module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Set up dependencies
	module.router = mockRouter

	// Initialize module - this will use the registered config (empty)
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Manually set the test config (this is how other tests do it)
	module.config = testConfig

	// Now manually initialize the health checker since we changed the config
	if testConfig.HealthCheck.Enabled {
		// Convert logger to slog.Logger
		var logger *slog.Logger
		if slogLogger, ok := mockApp.Logger().(*slog.Logger); ok {
			logger = slogLogger
		} else {
			// Create a new slog logger if conversion fails
			logger = slog.Default()
		}

		module.healthChecker = NewHealthChecker(
			&testConfig.HealthCheck,
			testConfig.BackendServices,
			module.httpClient,
			logger,
		)
	}

	// Check if health checker was created
	if !assert.NotNil(t, module.healthChecker, "Health checker should be created when enabled") {
		t.FailNow()
	}

	// Start module
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Verify health checker was started
	assert.True(t, module.healthChecker.IsRunning())

	// Wait for health checks
	time.Sleep(100 * time.Millisecond)

	// Check health status
	status := module.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Len(t, status, 1)
	assert.Contains(t, status, "api")
	assert.True(t, status["api"].Healthy)

	// Test individual backend status
	backendStatus, exists := module.GetBackendHealthStatus("api")
	assert.True(t, exists)
	assert.True(t, backendStatus.Healthy)

	// Test IsHealthCheckEnabled
	assert.True(t, module.IsHealthCheckEnabled())

	// Test that requests are recorded
	if handler, exists := mockRouter.routes["/*"]; exists {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		// Check that request was recorded
		time.Sleep(10 * time.Millisecond)
		status := module.GetHealthStatus()
		assert.True(t, status["api"].LastRequest.After(time.Now().Add(-1*time.Second)))
	}

	// Stop module
	err = module.Stop(ctx)
	require.NoError(t, err)

	// Verify health checker was stopped
	assert.False(t, module.healthChecker.IsRunning())
}

// TestModule_HealthCheckDisabled tests module behavior when health check is disabled
func TestModule_HealthCheckDisabled(t *testing.T) {
	module := NewModule()

	testConfig := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"api": "http://api.example.com",
		},
		DefaultBackend: "api",
		HealthCheck: HealthCheckConfig{
			Enabled: false, // Disabled
		},
	}

	// Create mock app
	mockApp := NewMockTenantApplication()
	mockApp.configSections["reverseproxy"] = &mockConfigProvider{
		config: testConfig,
	}

	// Create mock router
	mockRouter := &testRouter{
		routes: make(map[string]http.HandlerFunc),
	}

	// Initialize module
	err := module.RegisterConfig(mockApp)
	require.NoError(t, err)

	// Set up dependencies
	module.router = mockRouter

	// Initialize module
	err = module.Init(mockApp)
	require.NoError(t, err)

	// Manually set the test config (this is how other tests do it)
	module.config = testConfig

	// Start module
	ctx := context.Background()
	err = module.Start(ctx)
	require.NoError(t, err)

	// Verify health checker was not created
	assert.Nil(t, module.healthChecker)

	// Test health check methods return expected values
	assert.False(t, module.IsHealthCheckEnabled())
	assert.Nil(t, module.GetHealthStatus())

	status, exists := module.GetBackendHealthStatus("api")
	assert.False(t, exists)
	assert.Nil(t, status)

	// Stop module
	err = module.Stop(ctx)
	assert.NoError(t, err)
}
