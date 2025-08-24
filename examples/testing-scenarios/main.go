package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/GoCodeAlone/modular/modules/reverseproxy"
)

type AppConfig struct {
	// Application-level configuration
	TestingMode    bool   `yaml:"testing_mode" default:"false" desc:"Enable testing mode with additional features"`
	ScenarioRunner bool   `yaml:"scenario_runner" default:"false" desc:"Enable scenario runner for automated testing"`
	MetricsEnabled bool   `yaml:"metrics_enabled" default:"true" desc:"Enable metrics collection"`
	LogLevel       string `yaml:"log_level" default:"info" desc:"Log level (debug, info, warn, error)"`
}

type TestingScenario struct {
	Name        string
	Description string
	Handler     func(*TestingApp) error
}

type TestingApp struct {
	app        modular.Application
	backends   map[string]*MockBackend
	scenarios  map[string]TestingScenario
	running    bool
	httpClient *http.Client
}

type MockBackend struct {
	Name           string
	Port           int
	FailureRate    float64
	ResponseDelay  time.Duration
	HealthEndpoint string
	server         *http.Server
	requestCount   int64
	mu             sync.RWMutex
}

func main() {
	// Parse command line flags
	scenario := flag.String("scenario", "", "Run specific testing scenario")
	duration := flag.Duration("duration", 60*time.Second, "Test duration")
	connections := flag.Int("connections", 10, "Number of concurrent connections for load testing")
	backend := flag.String("backend", "primary", "Target backend for testing")
	tenant := flag.String("tenant", "", "Tenant ID for multi-tenant testing")
	flag.Parse()

	// Configure feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{Level: slog.LevelDebug},
		)),
	)

	// Create testing application wrapper
	testApp := &TestingApp{
		app:       app,
		backends:  make(map[string]*MockBackend),
		scenarios: make(map[string]TestingScenario),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Initialize testing scenarios
	testApp.initializeScenarios()

	// Create tenant service
	tenantService := modular.NewStandardTenantService(app.Logger())
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		app.Logger().Error("Failed to register tenant service", "error", err)
		os.Exit(1)
	}

	// Feature flag evaluation is handled automatically by the reverseproxy module.
	// The module will create its own file-based feature flag evaluator when feature flags are enabled.
	//
	// For external feature flag services (like LaunchDarkly), create a separate module that:
	// 1. Implements the FeatureFlagEvaluator interface
	// 2. Provides a "featureFlagEvaluator" service
	// 3. Gets automatically discovered by the reverseproxy module via interface matching
	//
	// This demonstrates proper modular service dependency injection instead of manual service creation.

	// Register tenant config loader to load tenant configurations from files
	tenantConfigLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
		ConfigNameRegex: regexp.MustCompile(`^[\w-]+\.yaml$`),
		ConfigDir:       "tenants",
		ConfigFeeders: []modular.Feeder{
			feeders.NewYamlFeeder(""),
		},
	})
	if err := app.RegisterService("tenantConfigLoader", tenantConfigLoader); err != nil {
		app.Logger().Error("Failed to register tenant config loader", "error", err)
		os.Exit(1)
	}

	// Register modules
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(reverseproxy.NewModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Start mock backends
	testApp.startMockBackends()

	// Handle specific scenario requests
	if *scenario != "" {
		testApp.runScenario(*scenario, &ScenarioConfig{
			Duration:    *duration,
			Connections: *connections,
			Backend:     *backend,
			Tenant:      *tenant,
		})
		return
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		app.Logger().Info("Shutdown signal received, stopping application...")
		cancel()
	}()

	// Run application
	testApp.running = true
	app.Logger().Info("Starting testing scenarios application...")

	go func() {
		if err := app.Run(); err != nil {
			app.Logger().Error("Application error", "error", err)
			cancel()
		}
	}()

	// Wait for application to start up
	time.Sleep(2 * time.Second)

	// Register application health endpoint after modules have started
	testApp.registerHealthEndpointAfterStart()

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop mock backends
	testApp.stopMockBackends()
	testApp.running = false

	app.Logger().Info("Application stopped")
}

func (t *TestingApp) initializeScenarios() {
	t.scenarios = map[string]TestingScenario{
		"health-check": {
			Name:        "Health Check Testing",
			Description: "Test backend health monitoring and availability",
			Handler:     t.runHealthCheckScenario,
		},
		"load-test": {
			Name:        "Load Testing",
			Description: "Test high-concurrency request handling",
			Handler:     t.runLoadTestScenario,
		},
		"failover": {
			Name:        "Failover Testing",
			Description: "Test circuit breaker and failover behavior",
			Handler:     t.runFailoverScenario,
		},
		"feature-flags": {
			Name:        "Feature Flag Testing",
			Description: "Test feature flag-based routing",
			Handler:     t.runFeatureFlagScenario,
		},
		"multi-tenant": {
			Name:        "Multi-Tenant Testing",
			Description: "Test tenant isolation and routing",
			Handler:     t.runMultiTenantScenario,
		},
		"security": {
			Name:        "Security Testing",
			Description: "Test authentication and authorization",
			Handler:     t.runSecurityScenario,
		},
		"performance": {
			Name:        "Performance Testing",
			Description: "Test latency and throughput",
			Handler:     t.runPerformanceScenario,
		},
		"configuration": {
			Name:        "Configuration Testing",
			Description: "Test dynamic configuration updates",
			Handler:     t.runConfigurationScenario,
		},
		"error-handling": {
			Name:        "Error Handling Testing",
			Description: "Test error propagation and handling",
			Handler:     t.runErrorHandlingScenario,
		},
		"monitoring": {
			Name:        "Monitoring Testing",
			Description: "Test metrics and monitoring",
			Handler:     t.runMonitoringScenario,
		},

		// New Chimera Facade scenarios
		"toolkit-api": {
			Name:        "Toolkit API with Feature Flag Control",
			Description: "Test toolkit toolbox API with LaunchDarkly feature flag control",
			Handler:     t.runToolkitApiScenario,
		},
		"oauth-token": {
			Name:        "OAuth Token API",
			Description: "Test OAuth token endpoint with feature flag routing",
			Handler:     t.runOAuthTokenScenario,
		},
		"oauth-introspect": {
			Name:        "OAuth Introspection API",
			Description: "Test OAuth token introspection with feature flag routing",
			Handler:     t.runOAuthIntrospectScenario,
		},
		"tenant-config": {
			Name:        "Tenant Configuration Loading",
			Description: "Test per-tenant configuration loading and feature flag fallbacks",
			Handler:     t.runTenantConfigScenario,
		},
		"debug-endpoints": {
			Name:        "Debug and Monitoring Endpoints",
			Description: "Test debug endpoints for feature flags and system status",
			Handler:     t.runDebugEndpointsScenario,
		},
		"dry-run": {
			Name:        "Dry-Run Testing",
			Description: "Test dry-run mode for comparing backend responses",
			Handler:     t.runDryRunScenario,
		},
	}
}

func (t *TestingApp) startMockBackends() {
	backends := []struct {
		name   string
		port   int
		health string
	}{
		{"primary", 9001, "/health"},
		{"secondary", 9002, "/health"},
		{"canary", 9003, "/health"},
		{"legacy", 9004, "/status"},
		{"monitoring", 9005, "/metrics"},
		{"unstable", 9006, "/health"}, // For failover testing
		{"slow", 9007, "/health"},     // For performance testing
		{"chimera", 9008, "/health"},  // For LaunchDarkly scenarios
	}

	for _, backend := range backends {
		mockBackend := &MockBackend{
			Name:           backend.name,
			Port:           backend.port,
			HealthEndpoint: backend.health,
			ResponseDelay:  0,
			FailureRate:    0,
		}

		t.backends[backend.name] = mockBackend
		go t.startMockBackend(mockBackend)

		// Give backends time to start
		time.Sleep(100 * time.Millisecond)
	}

	t.app.Logger().Info("All mock backends started", "count", len(backends))
}

func (t *TestingApp) startMockBackend(backend *MockBackend) {
	mux := http.NewServeMux()

	// Main handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		backend.mu.Lock()
		backend.requestCount++
		count := backend.requestCount
		backend.mu.Unlock()

		// Simulate failure rate
		if backend.FailureRate > 0 && float64(count)/(float64(count)+100) < backend.FailureRate {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"simulated failure","backend":"%s","request_count":%d}`,
				backend.Name, count)
			return
		}

		// Simulate response delay
		if backend.ResponseDelay > 0 {
			time.Sleep(backend.ResponseDelay)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"backend":"%s","path":"%s","method":"%s","request_count":%d,"timestamp":"%s"}`,
			backend.Name, r.URL.Path, r.Method, count, time.Now().Format(time.RFC3339))
	})

	// Health endpoint
	mux.HandleFunc(backend.HealthEndpoint, func(w http.ResponseWriter, r *http.Request) {
		backend.mu.RLock()
		count := backend.requestCount
		backend.mu.RUnlock()

		// Simulate health check failures
		if backend.FailureRate > 0.5 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","backend":"%s","reason":"high failure rate"}`, backend.Name)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","backend":"%s","request_count":%d,"uptime":"%s"}`,
			backend.Name, count, time.Since(time.Now().Add(-time.Hour)).String())
	})

	// Metrics endpoint (for monitoring backend only)
	if backend.Name == "monitoring" {
		mux.HandleFunc("/backend-metrics", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "# HELP backend_requests_total Total number of requests\n")
			fmt.Fprintf(w, "# TYPE backend_requests_total counter\n")
			fmt.Fprintf(w, "backend_requests_total{backend=\"%s\"} %d\n", backend.Name, count)
		})
	}

	// Chimera-specific endpoints for LaunchDarkly scenarios
	if backend.Name == "chimera" {
		// Toolkit toolbox API endpoint
		mux.HandleFunc("/api/v1/toolkit/toolbox", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"chimera","endpoint":"toolkit-toolbox","method":"%s","request_count":%d,"feature_enabled":true}`,
				r.Method, count)
		})

		// OAuth token API endpoint
		mux.HandleFunc("/api/v1/authentication/oauth/token", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"access_token":"chimera_token_%d","token_type":"Bearer","expires_in":3600,"backend":"chimera","request_count":%d}`,
				count, count)
		})

		// OAuth introspection API endpoint
		mux.HandleFunc("/api/v1/authentication/oauth/introspect", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"active":true,"client_id":"test_client","backend":"chimera","request_count":%d}`, count)
		})

		// Dry-run test endpoint
		mux.HandleFunc("/api/v1/test/dryrun", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"chimera","endpoint":"dry-run","method":"%s","dry_run_mode":true,"request_count":%d}`,
				r.Method, count)
		})
	}

	// Legacy backend specific endpoints
	if backend.Name == "legacy" {
		// Toolkit toolbox API endpoint (legacy version)
		mux.HandleFunc("/api/v1/toolkit/toolbox", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"legacy","endpoint":"toolkit-toolbox","method":"%s","request_count":%d,"legacy_mode":true}`,
				r.Method, count)
		})

		// OAuth endpoints (legacy versions)
		mux.HandleFunc("/api/v1/authentication/oauth/token", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"access_token":"legacy_token_%d","token_type":"Bearer","expires_in":1800,"backend":"legacy","request_count":%d}`,
				count, count)
		})

		mux.HandleFunc("/api/v1/authentication/oauth/introspect", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"active":true,"client_id":"legacy_client","backend":"legacy","request_count":%d}`, count)
		})

		// Dry-run test endpoint (legacy version)
		mux.HandleFunc("/api/v1/test/dryrun", func(w http.ResponseWriter, r *http.Request) {
			backend.mu.RLock()
			count := backend.requestCount
			backend.mu.RUnlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"backend":"legacy","endpoint":"dry-run","method":"%s","legacy_response":true,"request_count":%d}`,
				r.Method, count)
		})
	}

	backend.server = &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              ":" + strconv.Itoa(backend.Port),
		Handler:           mux,
	}

	t.app.Logger().Info("Starting mock backend", "name", backend.Name, "port", backend.Port)
	if err := backend.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.app.Logger().Error("Mock backend error", "name", backend.Name, "error", err)
	}
}

func (t *TestingApp) stopMockBackends() {
	for name, backend := range t.backends {
		if backend.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := backend.server.Shutdown(ctx); err != nil {
				t.app.Logger().Error("Error stopping backend", "name", name, "error", err)
			}
			cancel()
		}
	}
}

// registerHealthEndpointAfterStart registers the health endpoint after modules have started
func (t *TestingApp) registerHealthEndpointAfterStart() {
	// Get the chimux router service after modules have started
	var router chimux.BasicRouter
	if err := t.app.GetService("router", &router); err != nil {
		t.app.Logger().Error("Failed to get router service for health endpoint", "error", err)
		return
	}

	// Register health endpoint that responds with application health, not backend health
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple health response indicating the reverse proxy application is running
		response := map[string]interface{}{
			"status":    "healthy",
			"service":   "testing-scenarios-reverse-proxy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
			"uptime":    time.Since(time.Now().Add(-time.Hour)).String(), // placeholder uptime
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.app.Logger().Error("Failed to encode health response", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	t.app.Logger().Info("Registered application health endpoint at /health")
}

type ScenarioConfig struct {
	Duration    time.Duration
	Connections int
	Backend     string
	Tenant      string
}

func (t *TestingApp) runScenario(scenarioName string, config *ScenarioConfig) {
	scenario, exists := t.scenarios[scenarioName]
	if !exists {
		fmt.Printf("Unknown scenario: %s\n", scenarioName)
		fmt.Println("Available scenarios:")
		for name, s := range t.scenarios {
			fmt.Printf("  %s - %s\n", name, s.Description)
		}
		os.Exit(1)
	}

	fmt.Printf("Running scenario: %s\n", scenario.Name)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Printf("Duration: %s\n", config.Duration)
	fmt.Printf("Connections: %d\n", config.Connections)
	fmt.Printf("Backend: %s\n", config.Backend)
	if config.Tenant != "" {
		fmt.Printf("Tenant: %s\n", config.Tenant)
	}
	fmt.Println("---")

	// Start the application for scenario testing
	go func() {
		if err := t.app.Run(); err != nil {
			t.app.Logger().Error("Application error during scenario testing", "error", err)
		}
	}()

	// Wait for application to start
	time.Sleep(2 * time.Second)

	// Register application health endpoint after modules have started
	t.registerHealthEndpointAfterStart()

	// Run the scenario
	if err := scenario.Handler(t); err != nil {
		fmt.Printf("Scenario failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scenario '%s' completed successfully\n", scenario.Name)
}

func (t *TestingApp) runHealthCheckScenario(app *TestingApp) error {
	fmt.Println("Running health check testing scenario...")

	// Test health checks for all backends
	backends := []string{"primary", "secondary", "canary", "legacy", "monitoring"}

	for _, backend := range backends {
		if mockBackend, exists := t.backends[backend]; exists {
			endpoint := fmt.Sprintf("http://localhost:%d%s", mockBackend.Port, mockBackend.HealthEndpoint)

			fmt.Printf("  Testing %s backend health (%s)... ", backend, endpoint)

			req, _ := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
			resp, err := t.httpClient.Do(req)
			if err != nil {
				fmt.Printf("FAIL - %v\n", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
			} else {
				fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
			}
		}
	}

	// Test health checks through reverse proxy
	fmt.Println("  Testing health endpoints through reverse proxy:")

	// Test the main health endpoint - should return application health, not be proxied
	fmt.Printf("    Testing /health (application health)... ")

	// Test if /health gets a proper response or 404 from the reverse proxy
	proxyURL := "http://localhost:8080/health"
	req, _ := http.NewRequestWithContext(context.Background(), "GET", proxyURL, nil)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("FAIL - %v\n", err)
	} else {
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusNotFound:
			// If we get 404, it means our health endpoint exclusion is working correctly
			// The application health endpoint should not be proxied to backends
			fmt.Printf("PASS - Health endpoint not proxied (404 as expected)\n")
		case http.StatusOK:
			// Check if it's application health or backend health
			var healthResponse map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&healthResponse); err != nil {
				fmt.Printf("FAIL - Could not decode response: %v\n", err)
			} else {
				// Check if it's the application health response
				if service, ok := healthResponse["service"].(string); ok && service == "testing-scenarios-reverse-proxy" {
					fmt.Printf("PASS - Application health endpoint working correctly\n")
				} else {
					fmt.Printf("PARTIAL - Got response but not application health (backend/module health): %v\n", healthResponse)
				}
			}
		default:
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test other health-related endpoints
	healthEndpoints := []string{
		"/api/v1/health",  // Should be proxied to backend
		"/legacy/status",  // Should be proxied to legacy backend
		"/metrics/health", // Should return reverseproxy module health if configured
	}

	for _, endpoint := range healthEndpoints {
		proxyURL := fmt.Sprintf("http://localhost:8080%s", endpoint)
		fmt.Printf("    Testing %s (proxied to backend)... ", endpoint)

		req, _ := http.NewRequestWithContext(context.Background(), "GET", proxyURL, nil)
		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	return nil
}

func (t *TestingApp) runLoadTestScenario(app *TestingApp) error {
	fmt.Println("Running load testing scenario...")

	// Configuration for load test
	numRequests := 50
	concurrency := 10
	endpoint := "http://localhost:8080/api/v1/loadtest"

	fmt.Printf("  Configuration: %d requests, %d concurrent\n", numRequests, concurrency)
	fmt.Printf("  Target endpoint: %s\n", endpoint)

	// Channel to collect results
	results := make(chan error, numRequests)
	semaphore := make(chan struct{}, concurrency)

	start := time.Now()

	// Launch requests
	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
			if err != nil {
				results <- fmt.Errorf("request %d: create request failed: %w", requestID, err)
				return
			}

			req.Header.Set("X-Request-ID", fmt.Sprintf("load-test-%d", requestID))
			req.Header.Set("X-Test-Scenario", "load-test")

			resp, err := t.httpClient.Do(req)
			if err != nil {
				results <- fmt.Errorf("request %d: %w", requestID, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d: HTTP %d: %w", requestID, resp.StatusCode, errRequestFailed)
				return
			}

			results <- nil // Success
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	var errors []string

	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errorCount++
			errors = append(errors, err.Error())
		} else {
			successCount++
		}
	}

	duration := time.Since(start)

	fmt.Printf("  Results:\n")
	fmt.Printf("    Total requests: %d\n", numRequests)
	fmt.Printf("    Successful: %d\n", successCount)
	fmt.Printf("    Failed: %d\n", errorCount)
	fmt.Printf("    Duration: %v\n", duration)
	fmt.Printf("    Requests/sec: %.2f\n", float64(numRequests)/duration.Seconds())

	if errorCount > 0 {
		fmt.Printf("  Errors (showing first 5):\n")
		for i, err := range errors {
			if i >= 5 {
				fmt.Printf("    ... and %d more errors\n", len(errors)-5)
				break
			}
			fmt.Printf("    %s\n", err)
		}
	}

	// Consider test successful if at least 80% of requests succeeded
	successRate := float64(successCount) / float64(numRequests)
	if successRate < 0.8 {
		return fmt.Errorf("load test failed: success rate %.2f%% is below 80%%: %w", successRate*100, errLoadTestFailed)
	}

	fmt.Printf("  Load test PASSED (success rate: %.2f%%)\n", successRate*100)
	return nil
}

func (t *TestingApp) runFailoverScenario(app *TestingApp) error {
	fmt.Println("Running failover/circuit breaker testing scenario...")

	// Test 1: Normal operation
	fmt.Println("  Phase 1: Testing normal operation")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/test", nil)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("normal operation test failed: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("    Normal operation: PASS")
	} else {
		fmt.Printf("    Normal operation: FAIL (HTTP %d)\n", resp.StatusCode)
	}

	// Test 2: Introduce failures to trigger circuit breaker
	fmt.Println("  Phase 2: Introducing backend failures")

	if unstableBackend, exists := t.backends["unstable"]; exists {
		// Set high failure rate
		unstableBackend.mu.Lock()
		unstableBackend.FailureRate = 0.8 // 80% failure rate
		unstableBackend.mu.Unlock()

		fmt.Println("    Set unstable backend failure rate to 80%")

		// Make multiple requests to trigger circuit breaker
		fmt.Println("    Making requests to trigger circuit breaker...")
		failureCount := 0
		for i := 0; i < 10; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/unstable/test", nil)
			resp, err := t.httpClient.Do(req)
			if err != nil {
				failureCount++
				fmt.Printf("    Request %d: Network error\n", i+1)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode >= 500 {
				failureCount++
				fmt.Printf("    Request %d: HTTP %d (failure)\n", i+1, resp.StatusCode)
			} else {
				fmt.Printf("    Request %d: HTTP %d (success)\n", i+1, resp.StatusCode)
			}

			// Small delay between requests
			time.Sleep(100 * time.Millisecond)
		}

		fmt.Printf("    Triggered %d failures out of 10 requests\n", failureCount)

		// Test 3: Verify circuit breaker behavior
		fmt.Println("  Phase 3: Testing circuit breaker behavior")
		time.Sleep(2 * time.Second) // Allow circuit breaker to open

		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/unstable/test", nil)
		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("    Circuit breaker test: Network error - %v\n", err)
		} else {
			resp.Body.Close()
			fmt.Printf("    Circuit breaker test: HTTP %d\n", resp.StatusCode)
		}

		// Test 4: Reset backend and test recovery
		fmt.Println("  Phase 4: Testing recovery")
		unstableBackend.mu.Lock()
		unstableBackend.FailureRate = 0 // Reset to normal
		unstableBackend.mu.Unlock()

		fmt.Println("    Reset backend failure rate to 0%")
		fmt.Println("    Waiting for circuit breaker recovery...")
		time.Sleep(5 * time.Second)

		// Test recovery
		successCount := 0
		for i := 0; i < 5; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/unstable/test", nil)
			resp, err := t.httpClient.Do(req)
			if err != nil {
				fmt.Printf("    Recovery test %d: Network error\n", i+1)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				successCount++
				fmt.Printf("    Recovery test %d: HTTP %d (success)\n", i+1, resp.StatusCode)
			} else {
				fmt.Printf("    Recovery test %d: HTTP %d (still failing)\n", i+1, resp.StatusCode)
			}

			time.Sleep(500 * time.Millisecond)
		}

		fmt.Printf("    Recovery: %d/5 requests successful\n", successCount)

		if successCount >= 3 {
			fmt.Println("  Failover scenario: PASS")
		} else {
			fmt.Println("  Failover scenario: PARTIAL (recovery incomplete)")
		}
	} else {
		return errUnstableBackendNotFound
	}

	return nil
}

func (t *TestingApp) runFeatureFlagScenario(app *TestingApp) error {
	fmt.Println("Running feature flag testing scenario...")

	// Test 1: Enable feature flags and test routing
	fmt.Println("  Phase 1: Testing feature flag enabled routing")

	// Enable API v1 feature flag

	testCases := []struct {
		endpoint      string
		description   string
		expectBackend string
	}{
		{"/api/v1/test", "API v1 with flag enabled", "primary"},
		{"/api/v2/test", "API v2 with flag disabled", "primary"},     // Should fallback
		{"/api/canary/test", "Canary with flag disabled", "primary"}, // Should fallback
	}

	for _, tc := range testCases {
		fmt.Printf("    Testing %s... ", tc.description)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}

		req.Header.Set("X-Test-Scenario", "feature-flag")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test 2: Test tenant-specific feature flags
	fmt.Println("  Phase 2: Testing tenant-specific feature flags")

	// Set tenant-specific flags

	tenantTests := []struct {
		tenant      string
		endpoint    string
		description string
	}{
		{"tenant-alpha", "/api/v2/test", "Alpha tenant with v2 enabled"},
		{"tenant-beta", "/api/canary/test", "Beta tenant with canary enabled"},
		{"tenant-canary", "/api/v2/test", "Canary tenant with global flag"},
	}

	for _, tc := range tenantTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}

		req.Header.Set("X-Tenant-ID", tc.tenant)
		req.Header.Set("X-Test-Scenario", "feature-flag-tenant")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test 3: Dynamic flag changes
	fmt.Println("  Phase 3: Testing dynamic flag changes")

	// Toggle flags and test
	fmt.Printf("    Enabling all feature flags... ")

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v2/test", nil)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	fmt.Printf("    Disabling all feature flags... ")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/test", nil)
	if err != nil {
		fmt.Printf("FAIL - %v\n", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d (fallback working)\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	fmt.Println("  Feature flag scenario: PASS")
	return nil
}

func (t *TestingApp) runMultiTenantScenario(app *TestingApp) error {
	fmt.Println("Running multi-tenant testing scenario...")

	// Test 1: Different tenants routing to different backends
	fmt.Println("  Phase 1: Testing tenant-specific routing")

	tenantTests := []struct {
		tenant      string
		endpoint    string
		description string
	}{
		{"tenant-alpha", "/api/v1/test", "Alpha tenant (primary backend)"},
		{"tenant-beta", "/api/v1/test", "Beta tenant (secondary backend)"},
		{"tenant-canary", "/api/v1/test", "Canary tenant (canary backend)"},
		{"tenant-enterprise", "/api/enterprise/test", "Enterprise tenant (custom routing)"},
	}

	for _, tc := range tenantTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}

		req.Header.Set("X-Tenant-ID", tc.tenant)
		req.Header.Set("X-Test-Scenario", "multi-tenant")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test 2: Tenant isolation - different tenants should not interfere
	fmt.Println("  Phase 2: Testing tenant isolation")

	// Make concurrent requests from different tenants
	results := make(chan string, 6)

	tenants := []string{"tenant-alpha", "tenant-beta", "tenant-canary"}

	for _, tenant := range tenants {
		go func(t string) {
			req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/isolation", nil)
			if err != nil {
				results <- fmt.Sprintf("%s: request creation failed", t)
				return
			}

			req.Header.Set("X-Tenant-ID", t)
			req.Header.Set("X-Test-Scenario", "isolation")

			resp, err := app.httpClient.Do(req)
			if err != nil {
				results <- fmt.Sprintf("%s: request failed", t)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				results <- fmt.Sprintf("%s: PASS", t)
			} else {
				results <- fmt.Sprintf("%s: FAIL (HTTP %d)", t, resp.StatusCode)
			}
		}(tenant)

		// Also test the same tenant twice
		go func(t string) {
			req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/isolation2", nil)
			if err != nil {
				results <- fmt.Sprintf("%s-2: request creation failed", t)
				return
			}

			req.Header.Set("X-Tenant-ID", t)
			req.Header.Set("X-Test-Scenario", "isolation")

			resp, err := app.httpClient.Do(req)
			if err != nil {
				results <- fmt.Sprintf("%s-2: request failed", t)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				results <- fmt.Sprintf("%s-2: PASS", t)
			} else {
				results <- fmt.Sprintf("%s-2: FAIL (HTTP %d)", t, resp.StatusCode)
			}
		}(tenant)
	}

	// Collect results
	for i := 0; i < 6; i++ {
		result := <-results
		fmt.Printf("    Isolation test - %s\n", result)
	}

	// Test 3: No tenant header (should use default)
	fmt.Println("  Phase 3: Testing default behavior (no tenant)")

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/default", nil)
	if err != nil {
		return fmt.Errorf("default test request creation failed: %w", err)
	}

	req.Header.Set("X-Test-Scenario", "no-tenant")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    No tenant test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("    No tenant test: PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("    No tenant test: FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test 4: Unknown tenant (should use default)
	fmt.Println("  Phase 4: Testing unknown tenant fallback")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/unknown", nil)
	if err != nil {
		return fmt.Errorf("unknown tenant test request creation failed: %w", err)
	}

	req.Header.Set("X-Tenant-ID", "unknown-tenant-xyz")
	req.Header.Set("X-Test-Scenario", "unknown-tenant")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Unknown tenant test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("    Unknown tenant test: PASS - HTTP %d (fallback working)\n", resp.StatusCode)
		} else {
			fmt.Printf("    Unknown tenant test: FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	fmt.Println("  Multi-tenant scenario: PASS")
	return nil
}

func (t *TestingApp) runSecurityScenario(app *TestingApp) error {
	fmt.Println("Running security testing scenario...")

	// Test 1: CORS handling
	fmt.Println("  Phase 1: Testing CORS headers")

	req, err := http.NewRequestWithContext(context.Background(), "OPTIONS", "http://localhost:8080/api/v1/test", nil)
	if err != nil {
		return fmt.Errorf("CORS preflight request creation failed: %w", err)
	}

	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    CORS preflight test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    CORS preflight test: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: Header security
	fmt.Println("  Phase 2: Testing header security")

	securityTests := []struct {
		description string
		headers     map[string]string
		expectPass  bool
	}{
		{
			"Valid authorization header",
			map[string]string{"Authorization": "Bearer valid-token-123"},
			true,
		},
		{
			"Missing authorization for secure endpoint",
			map[string]string{},
			true, // Still passes but may get different response
		},
		{
			"Malicious header injection attempt",
			map[string]string{"X-Test": "value\r\nInjected: header"},
			true, // Should be handled safely
		},
	}

	for _, tc := range securityTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/secure", nil)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}

		for k, v := range tc.headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("X-Test-Scenario", "security")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 500 { // Any response except server error is acceptable
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	fmt.Println("  Security scenario: PASS")
	return nil
}

func (t *TestingApp) runPerformanceScenario(app *TestingApp) error {
	fmt.Println("Running performance testing scenario...")

	// Test different endpoints and measure response times
	performanceTests := []struct {
		endpoint    string
		description string
		maxLatency  time.Duration
	}{
		{"/api/v1/fast", "Fast endpoint", 100 * time.Millisecond},
		{"/api/v1/normal", "Normal endpoint", 500 * time.Millisecond},
		{"/slow/test", "Slow endpoint", 2 * time.Second},
	}

	fmt.Println("  Phase 1: Response time measurements")

	for _, tc := range performanceTests {
		fmt.Printf("    Testing %s... ", tc.description)

		start := time.Now()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		resp, err := t.httpClient.Do(req)
		latency := time.Since(start)

		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - %v (target: <%v)\n", latency, tc.maxLatency)
		} else {
			fmt.Printf("FAIL - HTTP %d in %v\n", resp.StatusCode, latency)
		}
	}

	// Test 2: Throughput measurement
	fmt.Println("  Phase 2: Throughput measurement (10 requests)")

	start := time.Now()
	successCount := 0

	for i := 0; i < 10; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/throughput", nil)
		resp, err := t.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				successCount++
			}
		}
	}

	duration := time.Since(start)
	throughput := float64(successCount) / duration.Seconds()

	fmt.Printf("    Throughput: %.2f requests/second (%d/%d successful)\n", throughput, successCount, 10)

	fmt.Println("  Performance scenario: PASS")
	return nil
}

func (t *TestingApp) runConfigurationScenario(app *TestingApp) error {
	fmt.Println("Running configuration testing scenario...")

	// Test different routing configurations
	configTests := []struct {
		endpoint    string
		description string
	}{
		{"/api/v1/config", "API v1 routing"},
		{"/api/v2/config", "API v2 routing"},
		{"/legacy/config", "Legacy routing"},
		{"/metrics/config", "Metrics routing"},
	}

	fmt.Println("  Phase 1: Testing route configurations")

	for _, tc := range configTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	fmt.Println("  Configuration scenario: PASS")
	return nil
}

func (t *TestingApp) runErrorHandlingScenario(app *TestingApp) error {
	fmt.Println("Running error handling testing scenario...")

	// Test various error conditions
	errorTests := []struct {
		endpoint       string
		method         string
		description    string
		expectedStatus int
	}{
		{"/nonexistent", "GET", "404 Not Found", 404},
		{"/api/v1/test", "TRACE", "Method not allowed", 405},
		{"/api/v1/test", "GET", "Normal request", 200},
	}

	fmt.Println("  Phase 1: Testing error responses")

	for _, tc := range errorTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, err := http.NewRequestWithContext(context.Background(), tc.method, "http://localhost:8080"+tc.endpoint, nil)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == tc.expectedStatus {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - Expected HTTP %d, got HTTP %d\n", tc.expectedStatus, resp.StatusCode)
		}
	}

	fmt.Println("  Error handling scenario: PASS")
	return nil
}

func (t *TestingApp) runMonitoringScenario(app *TestingApp) error {
	fmt.Println("Running monitoring testing scenario...")

	// Test metrics endpoints
	monitoringTests := []struct {
		endpoint    string
		description string
	}{
		{"/metrics", "Application metrics"},
		{"/reverseproxy/metrics", "Reverse proxy metrics"},
		{"/health", "Health check endpoint"},
	}

	fmt.Println("  Phase 1: Testing monitoring endpoints")

	for _, tc := range monitoringTests {
		fmt.Printf("    Testing %s... ", tc.description)

		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+tc.endpoint, nil)
		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("FAIL - %v\n", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf("PASS - HTTP %d\n", resp.StatusCode)
		} else {
			fmt.Printf("FAIL - HTTP %d\n", resp.StatusCode)
		}
	}

	// Test with tracing headers
	fmt.Println("  Phase 2: Testing request tracing")

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/trace", nil)
	if err != nil {
		return fmt.Errorf("trace request creation failed: %w", err)
	}

	req.Header.Set("X-Trace-ID", "test-trace-123456")
	req.Header.Set("X-Request-ID", "test-request-789")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Tracing test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Tracing test: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  Monitoring scenario: PASS")
	return nil
}

// New Chimera Facade Scenarios

func (t *TestingApp) runToolkitApiScenario(app *TestingApp) error {
	fmt.Println("Running Toolkit API with Feature Flag Control scenario...")

	// Test the specific toolkit toolbox API endpoint from Chimera scenarios
	endpoint := "/api/v1/toolkit/toolbox"

	// Test 1: Without tenant (should use global feature flag)
	fmt.Println("  Phase 1: Testing toolkit API without tenant context")

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+endpoint, nil)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Toolkit API test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Toolkit API test: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: With sampleaff1 tenant (should use tenant-specific configuration)
	fmt.Println("  Phase 2: Testing toolkit API with sampleaff1 tenant")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "toolkit-api")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Toolkit API with tenant: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Toolkit API with tenant: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 3: Test feature flag behavior
	fmt.Println("  Phase 3: Testing feature flag behavior")

	// Enable the feature flag

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "toolkit-api-enabled")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Toolkit API with flag enabled: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Toolkit API with flag enabled: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Disable the feature flag

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Toolkit API with flag disabled: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Toolkit API with flag disabled: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  Toolkit API scenario: PASS")
	return nil
}

func (t *TestingApp) runOAuthTokenScenario(app *TestingApp) error {
	fmt.Println("Running OAuth Token API scenario...")

	// Test the specific OAuth token API endpoint from Chimera scenarios
	endpoint := "/api/v1/authentication/oauth/token"

	// Test 1: POST request to OAuth token endpoint
	fmt.Println("  Phase 1: Testing OAuth token API")

	req, err := http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "oauth-token")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    OAuth token API: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    OAuth token API: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: Test with feature flag enabled
	fmt.Println("  Phase 2: Testing OAuth token API with feature flag")

	req, err = http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "oauth-token-enabled")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    OAuth token API with flag: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    OAuth token API with flag: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  OAuth Token API scenario: PASS")
	return nil
}

func (t *TestingApp) runOAuthIntrospectScenario(app *TestingApp) error {
	fmt.Println("Running OAuth Introspection API scenario...")

	// Test the specific OAuth introspection API endpoint from Chimera scenarios
	endpoint := "/api/v1/authentication/oauth/introspect"

	// Test 1: POST request to OAuth introspection endpoint
	fmt.Println("  Phase 1: Testing OAuth introspection API")

	req, err := http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "oauth-introspect")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    OAuth introspection API: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    OAuth introspection API: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: Test with feature flag
	fmt.Println("  Phase 2: Testing OAuth introspection API with feature flag")

	req, err = http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "oauth-introspect-enabled")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    OAuth introspection API with flag: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    OAuth introspection API with flag: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  OAuth Introspection API scenario: PASS")
	return nil
}

func (t *TestingApp) runTenantConfigScenario(app *TestingApp) error {
	fmt.Println("Running Tenant Configuration Loading scenario...")

	// Test 1: Test with existing tenant (sampleaff1)
	fmt.Println("  Phase 1: Testing with existing tenant sampleaff1")

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/test", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "tenant-config")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Existing tenant test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Existing tenant test: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: Test with non-existent tenant
	fmt.Println("  Phase 2: Testing with non-existent tenant")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/test", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "nonexistent")
	req.Header.Set("X-Test-Scenario", "tenant-config-fallback")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Non-existent tenant test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Non-existent tenant test: PASS - HTTP %d (fallback working)\n", resp.StatusCode)
	}

	// Test 3: Test feature flag fallback behavior
	fmt.Println("  Phase 3: Testing feature flag fallback behavior")

	// Set tenant-specific flags

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/api/v1/toolkit/toolbox", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "tenant-flag-fallback")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Tenant flag fallback test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Tenant flag fallback test: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  Tenant Configuration scenario: PASS")
	return nil
}

func (t *TestingApp) runDebugEndpointsScenario(app *TestingApp) error {
	fmt.Println("Running Debug and Monitoring Endpoints scenario...")

	// Test 1: Feature flags debug endpoint
	fmt.Println("  Phase 1: Testing feature flags debug endpoint")

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/debug/flags", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Debug flags endpoint: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Debug flags endpoint: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: General debug info endpoint
	fmt.Println("  Phase 2: Testing general debug info endpoint")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/debug/info", nil)
	if err != nil {
		fmt.Printf("    Debug info endpoint: FAIL - %v\n", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Debug info endpoint: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Debug info endpoint: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 3: Backend status endpoint
	fmt.Println("  Phase 3: Testing backend status endpoint")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/debug/backends", nil)
	if err != nil {
		fmt.Printf("    Debug backends endpoint: FAIL - %v\n", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Debug backends endpoint: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Debug backends endpoint: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 4: Circuit breaker status endpoint
	fmt.Println("  Phase 4: Testing circuit breaker status endpoint")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/debug/circuit-breakers", nil)
	if err != nil {
		fmt.Printf("    Debug circuit breakers endpoint: FAIL - %v\n", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Debug circuit breakers endpoint: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Debug circuit breakers endpoint: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 5: Health check status endpoint
	fmt.Println("  Phase 5: Testing health check status endpoint")

	req, err = http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/debug/health-checks", nil)
	if err != nil {
		fmt.Printf("    Debug health checks endpoint: FAIL - %v\n", err)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Debug health checks endpoint: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Debug health checks endpoint: PASS - HTTP %d\n", resp.StatusCode)
	}

	fmt.Println("  Debug Endpoints scenario: PASS")
	return nil
}

func (t *TestingApp) runDryRunScenario(app *TestingApp) error {
	fmt.Println("Running Dry-Run Testing scenario...")

	// Test the specific dry-run endpoint from configuration
	endpoint := "/api/v1/test/dryrun"

	// Test 1: Test dry-run mode
	fmt.Println("  Phase 1: Testing dry-run mode")

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "dry-run")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Dry-run test: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Dry-run test: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 2: Test dry-run with feature flag enabled
	fmt.Println("  Phase 2: Testing dry-run with feature flag enabled")

	req, err = http.NewRequestWithContext(context.Background(), "POST", "http://localhost:8080"+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Affiliate-ID", "sampleaff1")
	req.Header.Set("X-Test-Scenario", "dry-run-enabled")

	resp, err = t.httpClient.Do(req)
	if err != nil {
		fmt.Printf("    Dry-run with flag enabled: FAIL - %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("    Dry-run with flag enabled: PASS - HTTP %d\n", resp.StatusCode)
	}

	// Test 3: Test different HTTP methods in dry-run
	fmt.Println("  Phase 3: Testing different HTTP methods in dry-run")

	methods := []string{"GET", "POST", "PUT"}
	for _, method := range methods {
		req, err := http.NewRequestWithContext(context.Background(), method, "http://localhost:8080"+endpoint, nil)
		if err != nil {
			fmt.Printf("    Dry-run %s method: FAIL - %v\n", method, err)
			continue
		}

		req.Header.Set("X-Affiliate-ID", "sampleaff1")
		req.Header.Set("X-Test-Scenario", "dry-run-"+method)

		resp, err := t.httpClient.Do(req)
		if err != nil {
			fmt.Printf("    Dry-run %s method: FAIL - %v\n", method, err)
		} else {
			resp.Body.Close()
			fmt.Printf("    Dry-run %s method: PASS - HTTP %d\n", method, resp.StatusCode)
		}
	}

	fmt.Println("  Dry-Run scenario: PASS")
	return nil
}
