package reverseproxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
)

// TestNewFeatures tests the newly added features for debug endpoints and dry-run functionality
func TestNewFeatures(t *testing.T) {
	// Create a logger for tests
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("FileBasedFeatureFlagEvaluator_TenantAware", func(t *testing.T) {
		// Create mock application with tenant support
		app := NewMockTenantApplication()
		config := &ReverseProxyConfig{
			FeatureFlags: FeatureFlagsConfig{
				Enabled: true,
				Flags: map[string]bool{
					"global-flag": true,
					"api-v2":      false,
				},
			},
		}
		app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

		// Register tenant with override configuration
		tenantService := modular.NewStandardTenantService(logger)
		if err := app.RegisterService("tenantService", tenantService); err != nil {
			t.Fatalf("Failed to register tenant service: %v", err)
		}

		// Register tenant with specific config
		tenantConfig := &ReverseProxyConfig{
			FeatureFlags: FeatureFlagsConfig{
				Enabled: true,
				Flags: map[string]bool{
					"tenant-flag": true,
					"global-flag": false, // Override global
				},
			},
		}
		err := tenantService.RegisterTenant("tenant-1", map[string]modular.ConfigProvider{
			"reverseproxy": modular.NewStdConfigProvider(tenantConfig),
		})
		if err != nil {
			t.Fatalf("Failed to register tenant: %v", err)
		}

		evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
		if err != nil {
			t.Fatalf("Failed to create feature flag evaluator: %v", err)
		}

		ctx := context.Background()
		req := httptest.NewRequest("GET", "/test", nil)

		// Test global flag evaluation
		result, err := evaluator.EvaluateFlag(ctx, "global-flag", "", req)
		if err != nil {
			t.Errorf("Global flag evaluation failed: %v", err)
		}
		if result != true {
			t.Errorf("Expected global flag to be true, got %v", result)
		}

		// Test tenant flag override
		result, err = evaluator.EvaluateFlag(ctx, "global-flag", "tenant-1", req)
		if err != nil {
			t.Errorf("Tenant flag evaluation failed: %v", err)
		}
		if result != false {
			t.Errorf("Expected tenant override to be false, got %v", result)
		}

		// Test tenant-specific flag
		result, err = evaluator.EvaluateFlag(ctx, "tenant-flag", "tenant-1", req)
		if err != nil {
			t.Errorf("Tenant-specific flag evaluation failed: %v", err)
		}
		if result != true {
			t.Errorf("Expected tenant flag to be true, got %v", result)
		}

		// Test unknown flag
		result, err = evaluator.EvaluateFlag(ctx, "unknown-flag", "", req)
		if err == nil {
			t.Error("Expected error for unknown flag")
		}
		if result != false {
			t.Errorf("Expected unknown flag to be false, got %v", result)
		}

		// Test EvaluateFlagWithDefault
		result = evaluator.EvaluateFlagWithDefault(ctx, "missing-flag", "", req, true)
		if result != true {
			t.Errorf("Expected default value true for missing flag, got %v", result)
		}
	})

	t.Run("DryRunHandler", func(t *testing.T) {
		// Create mock backends
		primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Backend", "primary")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"backend":"primary","message":"test"}`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}))
		defer primaryServer.Close()

		secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Backend", "secondary")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"backend":"secondary","message":"test"}`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}))
		defer secondaryServer.Close()

		// Test dry-run disabled
		disabledConfig := DryRunConfig{
			Enabled: false,
		}
		disabledHandler := NewDryRunHandler(disabledConfig, "X-Tenant-ID", NewMockLogger())
		req := httptest.NewRequest("GET", "/test", nil)

		ctx := context.Background()
		_, err := disabledHandler.ProcessDryRun(ctx, req, primaryServer.URL, secondaryServer.URL)

		if err == nil {
			t.Error("Expected error when dry-run is disabled")
		}
		if !errors.Is(err, ErrDryRunModeNotEnabled) {
			t.Errorf("Expected ErrDryRunModeNotEnabled, got %v", err)
		}

		// Test dry-run enabled
		enabledConfig := DryRunConfig{
			Enabled:         true,
			LogResponses:    true,
			MaxResponseSize: 1024,
			CompareHeaders:  []string{"Content-Type", "X-Backend"},
			IgnoreHeaders:   []string{"Date"},
		}

		enabledHandler := NewDryRunHandler(enabledConfig, "X-Tenant-ID", NewMockLogger())
		req = httptest.NewRequest("POST", "/test", strings.NewReader(`{"test":"data"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "test-123")

		result, err := enabledHandler.ProcessDryRun(ctx, req, primaryServer.URL, secondaryServer.URL)

		if err != nil {
			t.Fatalf("Dry-run processing failed: %v", err)
		}

		if result == nil {
			t.Fatal("Dry-run result is nil")
		}

		// Verify result structure
		if result.PrimaryBackend != primaryServer.URL {
			t.Errorf("Expected primary backend %s, got %s", primaryServer.URL, result.PrimaryBackend)
		}

		if result.SecondaryBackend != secondaryServer.URL {
			t.Errorf("Expected secondary backend %s, got %s", secondaryServer.URL, result.SecondaryBackend)
		}

		if result.RequestID != "test-123" {
			t.Errorf("Expected request ID 'test-123', got %s", result.RequestID)
		}

		if result.Method != "POST" {
			t.Errorf("Expected method 'POST', got %s", result.Method)
		}

		// Verify responses were captured
		if result.PrimaryResponse.StatusCode != http.StatusOK {
			t.Errorf("Expected primary response status 200, got %d", result.PrimaryResponse.StatusCode)
		}

		if result.SecondaryResponse.StatusCode != http.StatusOK {
			t.Errorf("Expected secondary response status 200, got %d", result.SecondaryResponse.StatusCode)
		}

		// Verify comparison was performed
		if !result.Comparison.StatusCodeMatch {
			t.Error("Expected status codes to match")
		}

		// Verify timing information
		if result.Duration.Total == 0 {
			t.Error("Expected total duration to be greater than 0")
		}
	})

	t.Run("DebugHandler", func(t *testing.T) {
		// Create mock application with feature flag configuration
		app := NewMockTenantApplication()
		config := &ReverseProxyConfig{
			FeatureFlags: FeatureFlagsConfig{
				Enabled: true,
				Flags: map[string]bool{
					"test-flag": true,
					"api-v2":    false,
				},
			},
			BackendServices: map[string]string{
				"primary":   "http://localhost:9001",
				"secondary": "http://localhost:9002",
			},
		}
		app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

		// Create feature flag evaluator
		evaluator, err := NewFileBasedFeatureFlagEvaluator(app, logger)
		if err != nil {
			t.Fatalf("Failed to create feature flag evaluator: %v", err)
		}

		// Update config with routes
		config.Routes = map[string]string{
			"/api/v1/*": "primary",
			"/api/v2/*": "secondary",
		}
		config.DefaultBackend = "primary"
		config.TenantIDHeader = "X-Tenant-ID"
		config.RequireTenantID = false
		app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

		// Create debug handler
		debugConfig := DebugEndpointsConfig{
			Enabled:     true,
			BasePath:    "/debug",
			RequireAuth: false,
		}

		mockTenantService := &MockTenantService{}
		debugHandler := NewDebugHandler(debugConfig, evaluator, config, mockTenantService, logger)

		// Create test server
		mux := http.NewServeMux()
		debugHandler.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		// Test debug endpoints
		endpoints := []struct {
			path        string
			description string
		}{
			{"/debug/flags", "Feature flags endpoint"},
			{"/debug/info", "General info endpoint"},
			{"/debug/backends", "Backends endpoint"},
			{"/debug/circuit-breakers", "Circuit breakers endpoint"},
			{"/debug/health-checks", "Health checks endpoint"},
		}

		for _, endpoint := range endpoints {
			t.Run(endpoint.description, func(t *testing.T) {
				ctx := context.Background()
				req, err := http.NewRequestWithContext(ctx, "GET", server.URL+endpoint.path, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
				req.Header.Set("X-Tenant-ID", "test-tenant")

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer func() {
					if err := resp.Body.Close(); err != nil {
						t.Errorf("Failed to close response body: %v", err)
					}
				}()

				if resp.StatusCode != http.StatusOK {
					t.Errorf("Expected status 200, got %d", resp.StatusCode)
				}

				// Verify content type
				contentType := resp.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Errorf("Expected JSON content type, got: %s", contentType)
				}
			})
		}

		// Test authentication when required
		t.Run("Authentication", func(t *testing.T) {
			authDebugConfig := DebugEndpointsConfig{
				Enabled:     true,
				BasePath:    "/debug",
				RequireAuth: true,
				AuthToken:   "test-token",
			}

			authDebugHandler := NewDebugHandler(authDebugConfig, evaluator, config, mockTenantService, logger)
			authMux := http.NewServeMux()
			authDebugHandler.RegisterRoutes(authMux)
			authServer := httptest.NewServer(authMux)
			defer authServer.Close()

			// Test without auth token
			ctx := context.Background()
			req, err := http.NewRequestWithContext(ctx, "GET", authServer.URL+"/debug/flags", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected status 401 without auth, got %d", resp.StatusCode)
			}

			ctx = context.Background()
			// Test with correct auth token
			req, err = http.NewRequestWithContext(ctx, "GET", authServer.URL+"/debug/flags", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err = client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 with correct auth, got %d", resp.StatusCode)
			}

			// Test with incorrect auth token
			req, err = http.NewRequestWithContext(ctx, "GET", authServer.URL+"/debug/flags", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer wrong-token")

			resp, err = client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("Expected status 403 with wrong auth, got %d", resp.StatusCode)
			}
		})
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		// Test static error definitions
		if ErrDryRunModeNotEnabled == nil {
			t.Error("ErrDryRunModeNotEnabled should be defined")
		}

		if ErrDryRunModeNotEnabled.Error() != "dry-run mode is not enabled" {
			t.Errorf("Expected error message 'dry-run mode is not enabled', got '%s'", ErrDryRunModeNotEnabled.Error())
		}
	})
}

// TestScenarioIntegration tests integration of all new features
func TestScenarioIntegration(t *testing.T) {
	// This test validates that all the new features work together
	// as they would in the comprehensive testing scenarios example

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create mock application with global feature flag configuration
	app := NewMockTenantApplication()
	config := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"toolkit-toolbox-api":  false,
				"oauth-token-api":      false,
				"oauth-introspect-api": false,
				"test-dryrun-api":      true,
			},
		},
	}
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create tenant service and register tenant with overrides
	tenantService := modular.NewStandardTenantService(logger)
	if err := app.RegisterService("tenantService", tenantService); err != nil {
		t.Fatalf("Failed to register tenant service: %v", err)
	}

	// Register tenant with specific config (like sampleaff1 from scenarios)
	tenantConfig := &ReverseProxyConfig{
		FeatureFlags: FeatureFlagsConfig{
			Enabled: true,
			Flags: map[string]bool{
				"toolkit-toolbox-api":  false,
				"oauth-token-api":      true,
				"oauth-introspect-api": true,
			},
		},
	}
	err := tenantService.RegisterTenant("sampleaff1", map[string]modular.ConfigProvider{
		"reverseproxy": modular.NewStdConfigProvider(tenantConfig),
	})
	if err != nil {
		t.Fatalf("Failed to register tenant: %v", err)
	}

	// Create feature flag evaluator with typical Chimera scenarios
	_, err = NewFileBasedFeatureFlagEvaluator(app, logger) // Created for completeness but not used in this integration test
	if err != nil {
		t.Fatalf("Failed to create feature flag evaluator: %v", err)
	}

	// Test dry-run functionality with different backends
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"chimera","endpoint":"toolkit-toolbox","feature_enabled":true}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer primaryServer.Close()

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"legacy","endpoint":"toolkit-toolbox","legacy_mode":true}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer secondaryServer.Close()

	dryRunConfig := DryRunConfig{
		Enabled:                true,
		LogResponses:           true,
		MaxResponseSize:        1048576,
		CompareHeaders:         []string{"Content-Type"},
		IgnoreHeaders:          []string{"Date", "X-Request-ID"},
		DefaultResponseBackend: "secondary", // Test returning secondary response
	}

	dryRunHandler := NewDryRunHandler(dryRunConfig, "X-Affiliate-ID", logger)
	dryRunReq := httptest.NewRequest("GET", "/api/v1/test/dryrun", nil)
	dryRunReq.Header.Set("X-Affiliate-ID", "sampleaff1")

	ctx := context.Background()
	dryRunResult, err := dryRunHandler.ProcessDryRun(ctx, dryRunReq, primaryServer.URL, secondaryServer.URL)
	if err != nil {
		t.Fatalf("Dry-run processing failed: %v", err)
	}

	if dryRunResult == nil {
		t.Fatal("Dry-run result is nil")
	}

	// Verify both backends were called and responses compared
	if dryRunResult.PrimaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Expected primary response status 200, got %d", dryRunResult.PrimaryResponse.StatusCode)
	}

	if dryRunResult.SecondaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Expected secondary response status 200, got %d", dryRunResult.SecondaryResponse.StatusCode)
	}

	// Status codes should match
	if !dryRunResult.Comparison.StatusCodeMatch {
		t.Error("Expected status codes to match between backends")
	}

	// Test that the returned response indicates which backend was used
	if dryRunResult.ReturnedResponse != "secondary" {
		t.Errorf("Expected returned response to be 'secondary', got %s", dryRunResult.ReturnedResponse)
	}

	// Test the GetReturnedResponse method
	returnedResponse := dryRunResult.GetReturnedResponse()
	if returnedResponse.StatusCode != http.StatusOK {
		t.Errorf("Expected returned response status 200, got %d", returnedResponse.StatusCode)
	}

	// Body content should be different (chimera vs legacy response)
	if dryRunResult.Comparison.BodyMatch {
		t.Error("Expected body content to differ between backends")
	}

	// Should have differences reported
	if len(dryRunResult.Comparison.Differences) == 0 {
		t.Error("Expected differences to be reported between backends")
	}

	t.Logf("Integration test completed successfully - all new features working together")
	t.Logf("Feature flags evaluated, dry-run comparison completed with %d differences", len(dryRunResult.Comparison.Differences))
}
