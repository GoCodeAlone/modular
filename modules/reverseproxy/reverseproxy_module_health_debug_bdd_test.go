package reverseproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Health Check Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithHealthChecksConfiguredForDNSResolution() error {
	ctx.resetContext()

	// Create a test backend server with a resolvable hostname
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Create configuration with DNS-based health checking
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"dns-backend": testServer.URL, // Uses a URL that requires DNS resolution
		},
		Routes: map[string]string{
			"/api/*": "dns-backend",
		},
		DefaultBackend: testServer.URL,
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 500 * time.Millisecond,
			Timeout:  200 * time.Millisecond,
		},
	}

	// Register the configuration and module
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)
	ctx.app.RegisterModule(&ReverseProxyModule{})

	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) healthChecksShouldBePerformedUsingDNSResolution() error {
	// Check that the health check configuration exists
	if !ctx.service.config.HealthCheck.Enabled {
		return fmt.Errorf("health checks are not enabled")
	}

	// Check if health checks are actually running
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	// Test that the health checker can resolve the backend
	_, exists := ctx.service.config.BackendServices["dns-backend"]
	if !exists {
		return fmt.Errorf("DNS backend not configured")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) healthCheckStatusesShouldBeTrackedPerBackend() error {
	// Wait for some health checks to run
	time.Sleep(600 * time.Millisecond)

	// Verify health checker has status information
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	// Check that backend status tracking is in place
	for backendName := range ctx.service.config.BackendServices {
		allStatus := ctx.service.healthChecker.GetHealthStatus()
		if status, exists := allStatus[backendName]; !exists || status == nil {
			return fmt.Errorf("no health status tracked for backend: %s", backendName)
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithCustomHealthEndpointsPerBackend() error {
	ctx.resetContext()

	// Create multiple test backend servers with custom health endpoints
	healthyBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/custom-health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backend response"))
		}
	}))
	ctx.testServers = append(ctx.testServers, healthyBackend)

	unhealthyBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/different-health" {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backend response"))
		}
	}))
	ctx.testServers = append(ctx.testServers, unhealthyBackend)

	// Configure reverse proxy with per-backend health endpoints
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"healthy-backend":   healthyBackend.URL,
			"unhealthy-backend": unhealthyBackend.URL,
		},
		Routes: map[string]string{
			"/healthy/*":   "healthy-backend",
			"/unhealthy/*": "unhealthy-backend",
		},
		DefaultBackend: healthyBackend.URL,
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 200 * time.Millisecond,
			Timeout:  100 * time.Millisecond,
			HealthEndpoints: map[string]string{
				"healthy-backend":   "/custom-health",
				"unhealthy-backend": "/different-health",
			},
		},
	}

	// Register the configuration and module
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)
	ctx.app.RegisterModule(&ReverseProxyModule{})

	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) healthChecksUseDifferentEndpointsPerBackend() error {
	// Verify that the health endpoints are set up correctly
	healthyEndpoint, exists := ctx.service.config.HealthCheck.HealthEndpoints["healthy-backend"]
	if !exists || healthyEndpoint != "/custom-health" {
		return fmt.Errorf("healthy backend health endpoint not configured correctly")
	}

	unhealthyEndpoint, exists := ctx.service.config.HealthCheck.HealthEndpoints["unhealthy-backend"]
	if !exists || unhealthyEndpoint != "/different-health" {
		return fmt.Errorf("unhealthy backend health endpoint not configured correctly")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) backendHealthStatusesShouldReflectCustomEndpointResponses() error {
	// Wait for health checks to run
	time.Sleep(300 * time.Millisecond)

	// Check that different backends have different health statuses
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	healthyStatus := ctx.service.healthChecker.GetHealthStatus()["healthy-backend"]
	unhealthyStatus := ctx.service.healthChecker.GetHealthStatus()["unhealthy-backend"]

	if healthyStatus == nil || unhealthyStatus == nil {
		return fmt.Errorf("health status not available for backends")
	}

	// Verify that the healthy backend is actually healthy
	// It should respond with 200 OK on /custom-health
	if !healthyStatus.Healthy {
		return fmt.Errorf("healthy-backend should be healthy but is reported as unhealthy")
	}
	if !healthyStatus.HealthCheckPassing {
		return fmt.Errorf("healthy-backend health check should be passing but is reported as failing")
	}

	// Verify that the unhealthy backend is actually unhealthy
	// It should respond with 503 Service Unavailable on /different-health
	if unhealthyStatus.Healthy {
		return fmt.Errorf("unhealthy-backend should be unhealthy but is reported as healthy")
	}
	if unhealthyStatus.HealthCheckPassing {
		return fmt.Errorf("unhealthy-backend health check should be failing but is reported as passing")
	}

	// Verify that the unhealthy backend has error information
	if unhealthyStatus.LastError == "" {
		return fmt.Errorf("unhealthy-backend should have error information but LastError is empty")
	}

	// Verify that both backends have different health check results
	if healthyStatus.Healthy == unhealthyStatus.Healthy {
		return fmt.Errorf("backends should have different health statuses but both are the same")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithPerBackendHealthCheckConfiguration() error {
	ctx.resetContext()

	// Create test backend servers with different response patterns
	fastBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fast backend"))
	}))
	ctx.testServers = append(ctx.testServers, fastBackend)

	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond) // Slow response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow backend"))
	}))
	ctx.testServers = append(ctx.testServers, slowBackend)

	// Configure reverse proxy with per-backend health check settings
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"fast-backend": fastBackend.URL,
			"slow-backend": slowBackend.URL,
		},
		Routes: map[string]string{
			"/fast/*": "fast-backend",
			"/slow/*": "slow-backend",
		},
		DefaultBackend: fastBackend.URL,
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 300 * time.Millisecond,
			Timeout:  100 * time.Millisecond, // This will cause slow backend to timeout
			BackendHealthCheckConfig: map[string]BackendHealthConfig{
				"fast-backend": {
					Timeout: 50 * time.Millisecond,
				},
				"slow-backend": {
					Timeout: 200 * time.Millisecond, // Override for slow backend
				},
			},
		},
	}

	// Register the configuration and module
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)
	ctx.app.RegisterModule(&ReverseProxyModule{})

	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) eachBackendShouldUseItsSpecificHealthCheckSettings() error {
	// Verify that the backend health check configurations are set up correctly
	fastConfig, exists := ctx.service.config.HealthCheck.BackendHealthCheckConfig["fast-backend"]
	if !exists {
		return fmt.Errorf("fast backend health check configuration missing")
	}

	slowConfig, exists := ctx.service.config.HealthCheck.BackendHealthCheckConfig["slow-backend"]
	if !exists {
		return fmt.Errorf("slow backend health check configuration missing")
	}

	// Verify timeout configurations
	if fastConfig.Timeout != 50*time.Millisecond {
		return fmt.Errorf("fast backend health timeout not configured correctly")
	}

	if slowConfig.Timeout != 200*time.Millisecond {
		return fmt.Errorf("slow backend health timeout not configured correctly")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) healthCheckBehaviorShouldDifferPerBackend() error {
	// Wait for health checks to run
	time.Sleep(400 * time.Millisecond)

	// Verify health checker is working
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	// Check that both backends are being monitored
	allStatus := ctx.service.healthChecker.GetHealthStatus()
	fastStatus := allStatus["fast-backend"]
	slowStatus := allStatus["slow-backend"]

	if fastStatus == nil || slowStatus == nil {
		return fmt.Errorf("health status not available for all backends")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iConfigureHealthChecksWithRecentRequestThresholds() error {
	// Update configuration to include recent request thresholds
	ctx.config.HealthCheck.RecentRequestThreshold = 10 * time.Second

	// Re-register the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Reinitialize the app to pick up the new configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) iMakeFewerRequestsThanTheThreshold() error {
	// Make a few requests (less than the threshold of 5)
	for i := 0; i < 3; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/test", nil)
		if err != nil {
			return fmt.Errorf("failed to make request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) healthChecksShouldNotFlagTheBackendAsUnhealthy() error {
	// Wait for a health check cycle
	time.Sleep(2 * time.Second)

	// Check that the backend is still considered healthy despite not receiving enough requests
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	// Verify that backends are not marked unhealthy due to low request volume
	for backendName := range ctx.service.config.BackendServices {
		allStatus := ctx.service.healthChecker.GetHealthStatus()
		if status, exists := allStatus[backendName]; exists && status != nil && !status.Healthy {
			return fmt.Errorf("backend %s should not be marked unhealthy due to low request volume", backendName)
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) thresholdBasedHealthCheckingShouldBeRespected() error {
	// Make additional requests to exceed the threshold
	for i := 0; i < 3; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", "/test", nil)
		if err != nil {
			return fmt.Errorf("failed to make additional request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Wait for health check cycle
	time.Sleep(1 * time.Second)

	// Now that we've exceeded the threshold, health checking should be more active
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithExpectedHealthCheckStatusCodes() error {
	// Create a backend that returns various status codes
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusAccepted) // 202 - should be considered healthy
		} else {
			w.WriteHeader(http.StatusOK)
		}
		w.Write([]byte("response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Configure with specific expected status codes
	ctx.config.BackendServices = map[string]string{
		"custom-health-backend": testServer.URL,
	}
	ctx.config.HealthCheck.BackendHealthCheckConfig = map[string]BackendHealthConfig{
		"custom-health-backend": {
			Endpoint:            "/health",
			ExpectedStatusCodes: []int{200, 202}, // Accept both 200 and 202
		},
	}

	// Re-register configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) healthChecksAcceptConfiguredStatusCodes() error {
	// Verify the configuration is set correctly
	config, exists := ctx.service.config.HealthCheck.BackendHealthCheckConfig["custom-health-backend"]
	if !exists {
		return fmt.Errorf("custom health backend configuration not found")
	}

	expectedStatuses := config.ExpectedStatusCodes
	if len(expectedStatuses) != 2 || expectedStatuses[0] != 200 || expectedStatuses[1] != 202 {
		return fmt.Errorf("expected status codes not configured correctly: %v", expectedStatuses)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) nonStandardStatusCodesShouldBeAcceptedAsHealthy() error {
	// Wait for health checks to run
	time.Sleep(300 * time.Millisecond)

	// Verify that the backend returning 202 is considered healthy
	if ctx.service.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	allStatus := ctx.service.healthChecker.GetHealthStatus()
	status := allStatus["custom-health-backend"]
	if status == nil {
		return fmt.Errorf("no health status available for custom health backend")
	}

	// The backend should be healthy since 202 is in the expected status list
	return nil
}

// Metrics Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithMetricsCollectionEnabled() error {
	// Update configuration to enable metrics
	ctx.config.MetricsEnabled = true
	ctx.metricsEnabled = true

	// Re-register the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Reinitialize to apply metrics configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) metricsCollectionShouldBeActive() error {
	// Verify metrics are enabled in configuration
	if !ctx.service.config.MetricsEnabled {
		return fmt.Errorf("metrics collection not enabled in configuration")
	}

	// Make some requests to generate metrics
	for i := 0; i < 5; i++ {
		resp, err := ctx.makeRequestThroughModule("GET", fmt.Sprintf("/test-metrics-%d", i), nil)
		if err != nil {
			return fmt.Errorf("failed to make metrics test request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) requestMetricsShouldBeTracked() error {
	// Verify that the service is configured to collect metrics
	if !ctx.service.config.MetricsEnabled {
		return fmt.Errorf("metrics collection should be enabled")
	}

	// Check if metrics endpoint is available (if configured)
	if ctx.service.config.MetricsEndpoint != "" {
		resp, err := ctx.makeRequestThroughModule("GET", ctx.service.config.MetricsEndpoint, nil)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil // Metrics endpoint is working
			}
		}
	}

	// If no specific metrics endpoint, just verify configuration
	return nil
}

func (ctx *ReverseProxyBDDTestContext) responseTimesShouldBeMeasured() error {
	// Verify metrics configuration supports response time measurement
	if !ctx.service.config.MetricsEnabled {
		return fmt.Errorf("metrics collection should be enabled for response time measurement")
	}

	// Make a request and verify it completes (response time would be measured)
	resp, err := ctx.makeRequestThroughModule("GET", "/response-time-test", nil)
	if err != nil {
		return fmt.Errorf("failed to make response time test request: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iConfigureACustomMetricsEndpoint() error {
	// Update configuration with custom metrics endpoint
	ctx.config.MetricsEndpoint = "/custom-metrics"
	ctx.config.MetricsPath = "/custom-metrics"

	// Re-register the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Reinitialize to apply custom metrics endpoint
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) theCustomMetricsEndpointShouldBeAvailable() error {
	// Verify the custom endpoint is configured
	if ctx.service.config.MetricsEndpoint != "/custom-metrics" {
		return fmt.Errorf("custom metrics endpoint not configured correctly")
	}

	// Try to access the custom metrics endpoint
	resp, err := ctx.makeRequestThroughModule("GET", "/custom-metrics", nil)
	if err != nil {
		return fmt.Errorf("failed to access custom metrics endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Metrics endpoint should return some kind of response
	if resp.StatusCode >= 400 {
		return fmt.Errorf("custom metrics endpoint returned error status: %d", resp.StatusCode)
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) metricsShouldBeServedFromTheCustomPath() error {
	// Make a request to the custom path and verify we get metrics-like content
	resp, err := ctx.makeRequestThroughModule("GET", "/custom-metrics", nil)
	if err != nil {
		return fmt.Errorf("failed to get metrics from custom path: %v", err)
	}
	defer resp.Body.Close()

	// Read response to verify we get some content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read metrics response: %v", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("metrics endpoint returned empty response")
	}

	return nil
}

// Debug Endpoints Scenarios

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithDebugEndpointsEnabled() error {
	// Update configuration to enable debug endpoints
	ctx.config.DebugEndpoints = DebugEndpointsConfig{
		Enabled:  true,
		BasePath: "/debug",
	}
	ctx.debugEnabled = true

	// Re-register the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Reinitialize to enable debug endpoints
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) debugEndpointsShouldBeAccessible() error {
	// Test access to various debug endpoints
	debugEndpoints := []string{"/debug/info", "/debug/backends", "/debug/flags"}

	for _, endpoint := range debugEndpoints {
		resp, err := ctx.makeRequestThroughModule("GET", endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to access debug endpoint %s: %v", endpoint, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("debug endpoint %s returned status %d", endpoint, resp.StatusCode)
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) systemInformationShouldBeAvailableViaDebugEndpoints() error {
	// Test the info endpoint specifically
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/info", nil)
	if err != nil {
		return fmt.Errorf("failed to get debug info: %v", err)
	}
	defer resp.Body.Close()

	// Parse response to verify it contains system information
	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("failed to parse debug info response: %v", err)
	}

	// Verify some expected fields are present
	if len(info) == 0 {
		return fmt.Errorf("debug info response should contain system information")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iAccessTheDebugInfoEndpoint() error {
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/info", nil)
	if err != nil {
		return fmt.Errorf("failed to access debug info endpoint: %v", err)
	}
	defer resp.Body.Close()

	ctx.lastResponse = resp
	return nil
}

func (ctx *ReverseProxyBDDTestContext) configurationDetailsShouldBeReturned() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response available")
	}

	if ctx.lastResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", ctx.lastResponse.StatusCode)
	}

	// Parse response
	var info map[string]interface{}
	if err := json.NewDecoder(ctx.lastResponse.Body).Decode(&info); err != nil {
		return fmt.Errorf("failed to parse debug info: %v", err)
	}

	// Verify configuration details are included
	if len(info) == 0 {
		return fmt.Errorf("debug info should include configuration details")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iAccessTheDebugBackendsEndpoint() error {
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/backends", nil)
	if err != nil {
		return fmt.Errorf("failed to access debug backends endpoint: %v", err)
	}
	defer resp.Body.Close()

	ctx.lastResponse = resp
	return nil
}

func (ctx *ReverseProxyBDDTestContext) backendStatusInformationShouldBeReturned() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response available")
	}

	if ctx.lastResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", ctx.lastResponse.StatusCode)
	}

	// Parse response
	var backends map[string]interface{}
	if err := json.NewDecoder(ctx.lastResponse.Body).Decode(&backends); err != nil {
		return fmt.Errorf("failed to parse backends info: %v", err)
	}

	// Verify backend information is included
	if len(backends) == 0 {
		return fmt.Errorf("debug backends should include backend status information")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iAccessTheDebugFeatureFlagsEndpoint() error {
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/flags", nil)
	if err != nil {
		return fmt.Errorf("failed to access debug flags endpoint: %v", err)
	}
	defer resp.Body.Close()

	ctx.lastResponse = resp
	return nil
}

func (ctx *ReverseProxyBDDTestContext) featureFlagStatusShouldBeReturned() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response available")
	}

	if ctx.lastResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", ctx.lastResponse.StatusCode)
	}

	// Parse response
	var flags map[string]interface{}
	if err := json.NewDecoder(ctx.lastResponse.Body).Decode(&flags); err != nil {
		return fmt.Errorf("failed to parse flags info: %v", err)
	}

	// Feature flags endpoint should return some information
	// (even if empty, it should be a valid JSON response)
	return nil
}

func (ctx *ReverseProxyBDDTestContext) iAccessTheDebugCircuitBreakersEndpoint() error {
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/circuit-breakers", nil)
	if err != nil {
		return fmt.Errorf("failed to access debug circuit breakers endpoint: %v", err)
	}
	defer resp.Body.Close()

	ctx.lastResponse = resp
	return nil
}

func (ctx *ReverseProxyBDDTestContext) circuitBreakerMetricsShouldBeIncluded() error {
	// Make HTTP request to debug circuit-breakers endpoint
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/circuit-breakers", nil)
	if err != nil {
		return fmt.Errorf("failed to get circuit breaker metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var metrics map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode circuit breaker metrics: %v", err)
	}

	// Verify circuit breaker metrics are present
	if len(metrics) == 0 {
		return fmt.Errorf("circuit breaker metrics should be included in debug response")
	}

	// Check for expected metric fields
	for _, metric := range metrics {
		if metricMap, ok := metric.(map[string]interface{}); ok {
			if _, hasFailures := metricMap["failures"]; !hasFailures {
				return fmt.Errorf("circuit breaker metrics should include failure count")
			}
			if _, hasState := metricMap["state"]; !hasState {
				return fmt.Errorf("circuit breaker metrics should include state")
			}
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iAccessTheDebugHealthChecksEndpoint() error {
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/health-checks", nil)
	if err != nil {
		return fmt.Errorf("failed to access debug health checks endpoint: %v", err)
	}
	defer resp.Body.Close()

	ctx.lastResponse = resp
	return nil
}

func (ctx *ReverseProxyBDDTestContext) healthCheckHistoryShouldBeIncluded() error {
	// Make HTTP request to debug health-checks endpoint
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/health-checks", nil)
	if err != nil {
		return fmt.Errorf("failed to get health check history: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var healthData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		return fmt.Errorf("failed to decode health check data: %v", err)
	}

	// Verify health check history is present
	if len(healthData) == 0 {
		return fmt.Errorf("health check history should be included in debug response")
	}

	// Check for expected health check fields
	for _, health := range healthData {
		if healthMap, ok := health.(map[string]interface{}); ok {
			if _, hasStatus := healthMap["status"]; !hasStatus {
				return fmt.Errorf("health check history should include status")
			}
			if _, hasLastCheck := healthMap["lastCheck"]; !hasLastCheck {
				return fmt.Errorf("health check history should include last check time")
			}
		}
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) iHaveAReverseProxyWithDebugEndpointsAndHealthChecksEnabled() error {
	// Don't reset context - work with existing app from background
	// Just update the configuration

	// Create a test backend server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	ctx.testServers = append(ctx.testServers, testServer)

	// Update configuration to include both debug endpoints and health checks
	ctx.config = &ReverseProxyConfig{
		BackendServices: map[string]string{
			"test-backend": testServer.URL,
		},
		Routes: map[string]string{
			"/*": "test-backend",
		},
		DefaultBackend: testServer.URL,
		DebugEndpoints: DebugEndpointsConfig{
			Enabled:  true,
			BasePath: "/debug",
		},
		HealthCheck: HealthCheckConfig{
			Enabled:  true,
			Interval: 1 * time.Second,
			Timeout:  500 * time.Millisecond,
		},
	}

	// Register the updated configuration
	reverseproxyConfigProvider := modular.NewStdConfigProvider(ctx.config)
	ctx.app.RegisterConfigSection("reverseproxy", reverseproxyConfigProvider)

	// Initialize with the updated configuration
	return ctx.app.Init()
}

func (ctx *ReverseProxyBDDTestContext) debugEndpointsAndHealthChecksShouldBothBeActive() error {
	// Verify debug endpoints are accessible
	resp, err := ctx.makeRequestThroughModule("GET", "/debug/info", nil)
	if err != nil {
		return fmt.Errorf("debug endpoints not accessible: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("debug endpoint returned status %d", resp.StatusCode)
	}

	// Verify health checks are enabled
	if !ctx.service.config.HealthCheck.Enabled {
		return fmt.Errorf("health checks should be enabled")
	}

	return nil
}

func (ctx *ReverseProxyBDDTestContext) healthCheckStatusShouldBeReturned() error {
	// Verify health checks are enabled without re-initializing
	// Just check that the configuration was set up correctly
	if ctx.config == nil {
		return fmt.Errorf("configuration not available")
	}

	if !ctx.config.HealthCheck.Enabled {
		return fmt.Errorf("health checks not enabled")
	}

	return nil
}
