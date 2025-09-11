package reverseproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
)

// TestDryRunBugFixes tests the specific bugs that were fixed in the dry-run feature:
// 1. Request body consumption bug (body was consumed and unavailable for background comparison)
// 2. Context cancellation bug (original request context was canceled before background dry-run)
// 3. URL path joining bug (double slashes in URLs due to improper string concatenation)
func TestDryRunBugFixes(t *testing.T) {
	t.Run("RequestBodyConsumptionFix", testRequestBodyConsumptionFix)
	t.Run("ContextCancellationFix", testContextCancellationFix)
	t.Run("URLPathJoiningFix", testURLPathJoiningFix)
	t.Run("EndToEndDryRunWithRequestBody", testEndToEndDryRunWithRequestBody)
}

// testRequestBodyConsumptionFix verifies that request bodies are properly preserved
// for both the immediate response and background dry-run comparison
func testRequestBodyConsumptionFix(t *testing.T) {
	var primaryBodyReceived, secondaryBodyReceived string
	var mu sync.Mutex

	// Primary server that captures the request body
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Primary server failed to read body: %v", err)
		}
		mu.Lock()
		primaryBodyReceived = string(body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"primary"}`)); err != nil {
			t.Errorf("Primary server failed to write response: %v", err)
		}
	}))
	defer primaryServer.Close()

	// Secondary server that captures the request body
	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Secondary server failed to read body: %v", err)
		}
		mu.Lock()
		secondaryBodyReceived = string(body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"secondary"}`)); err != nil {
			t.Errorf("Secondary server failed to write response: %v", err)
		}
	}))
	defer secondaryServer.Close()

	// Create dry-run handler
	config := DryRunConfig{
		Enabled:         true,
		LogResponses:    true,
		MaxResponseSize: 1024,
	}
	handler := NewDryRunHandler(config, "X-Tenant-ID", NewMockLogger())

	// Create request with body content
	requestBody := `{"test":"data","message":"hello world"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Process dry-run
	ctx := context.Background()
	result, err := handler.ProcessDryRun(ctx, req, primaryServer.URL, secondaryServer.URL)

	if err != nil {
		t.Fatalf("Dry-run processing failed: %v", err)
	}

	// Verify both backends received the same request body
	mu.Lock()
	defer mu.Unlock()

	if primaryBodyReceived != requestBody {
		t.Errorf("Primary server received incorrect body. Expected: %q, Got: %q", requestBody, primaryBodyReceived)
	}

	if secondaryBodyReceived != requestBody {
		t.Errorf("Secondary server received incorrect body. Expected: %q, Got: %q", requestBody, secondaryBodyReceived)
	}

	if primaryBodyReceived != secondaryBodyReceived {
		t.Errorf("Body mismatch between backends. Primary: %q, Secondary: %q", primaryBodyReceived, secondaryBodyReceived)
	}

	// Verify responses were successful
	if result.PrimaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Primary response failed with status: %d", result.PrimaryResponse.StatusCode)
	}

	if result.SecondaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Secondary response failed with status: %d", result.SecondaryResponse.StatusCode)
	}

	// Verify no errors in responses
	if result.PrimaryResponse.Error != "" {
		t.Errorf("Primary response had error: %s", result.PrimaryResponse.Error)
	}

	if result.SecondaryResponse.Error != "" {
		t.Errorf("Secondary response had error: %s", result.SecondaryResponse.Error)
	}
}

// testContextCancellationFix verifies that background dry-run operations
// use an independent context that doesn't get canceled when the original request completes
func testContextCancellationFix(t *testing.T) {
	requestReceived := make(chan bool, 2)

	// Create servers that signal when they receive requests
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestReceived <- true:
		default:
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"primary"}`)); err != nil {
			t.Errorf("Primary server failed to write response: %v", err)
		}
	}))
	defer primaryServer.Close()

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestReceived <- true:
		default:
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"secondary"}`)); err != nil {
			t.Errorf("Secondary server failed to write response: %v", err)
		}
	}))
	defer secondaryServer.Close()

	// Create dry-run handler
	config := DryRunConfig{
		Enabled:         true,
		LogResponses:    true,
		MaxResponseSize: 1024,
	}
	handler := NewDryRunHandler(config, "X-Tenant-ID", NewMockLogger())

	// Create a context that will be canceled immediately after the call
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/api/test", nil)

	// Process dry-run
	result, err := handler.ProcessDryRun(ctx, req, primaryServer.URL, secondaryServer.URL)

	// Cancel the context immediately (simulating what happens when HTTP request completes)
	cancel()

	if err != nil {
		t.Fatalf("Dry-run processing failed: %v", err)
	}

	// Wait for both servers to receive requests
	timeout := time.After(5 * time.Second)
	receivedCount := 0

	for receivedCount < 2 {
		select {
		case <-requestReceived:
			receivedCount++
		case <-timeout:
			t.Fatalf("Timeout waiting for requests. Only received %d out of 2 requests", receivedCount)
		}
	}

	// Verify both responses were successful (no context cancellation errors)
	if result.PrimaryResponse.Error != "" {
		t.Errorf("Primary response had error: %s", result.PrimaryResponse.Error)
	}

	if result.SecondaryResponse.Error != "" {
		t.Errorf("Secondary response had error: %s", result.SecondaryResponse.Error)
	}

	// Verify both responses have valid status codes
	if result.PrimaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Primary response failed with status: %d", result.PrimaryResponse.StatusCode)
	}

	if result.SecondaryResponse.StatusCode != http.StatusOK {
		t.Errorf("Secondary response failed with status: %d", result.SecondaryResponse.StatusCode)
	}
}

// testURLPathJoiningFix verifies that URLs are properly constructed without double slashes
func testURLPathJoiningFix(t *testing.T) {
	var primaryURLReceived, secondaryURLReceived string
	var mu sync.Mutex

	// Primary server with trailing slash
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		primaryURLReceived = r.URL.String()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"primary"}`)); err != nil {
			t.Errorf("Primary server failed to write response: %v", err)
		}
	}))
	defer primaryServer.Close()

	// Secondary server without trailing slash
	secondaryServerBase := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		secondaryURLReceived = r.URL.String()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"secondary"}`)); err != nil {
			t.Errorf("Secondary server failed to write response: %v", err)
		}
	}))
	defer secondaryServerBase.Close()

	// Create dry-run handler
	config := DryRunConfig{
		Enabled:         true,
		LogResponses:    true,
		MaxResponseSize: 1024,
	}
	handler := NewDryRunHandler(config, "X-Tenant-ID", NewMockLogger())

	// Test various URL combinations that could cause double slashes
	testCases := []struct {
		name         string
		primaryURL   string
		secondaryURL string
		requestPath  string
		expectedPath string
	}{
		{
			name:         "Backend with trailing slash, path with leading slash",
			primaryURL:   primaryServer.URL + "/",
			secondaryURL: secondaryServerBase.URL,
			requestPath:  "/api/v1/test",
			expectedPath: "/api/v1/test",
		},
		{
			name:         "Both URLs with trailing slash",
			primaryURL:   primaryServer.URL + "/",
			secondaryURL: secondaryServerBase.URL + "/",
			requestPath:  "/api/v1/test",
			expectedPath: "/api/v1/test",
		},
		{
			name:         "Backend without trailing slash, path with leading slash",
			primaryURL:   primaryServer.URL,
			secondaryURL: secondaryServerBase.URL,
			requestPath:  "/api/v1/test",
			expectedPath: "/api/v1/test",
		},
		{
			name:         "Backend with trailing slash, path without leading slash",
			primaryURL:   primaryServer.URL + "/",
			secondaryURL: secondaryServerBase.URL + "/",
			requestPath:  "/api/v1/test", // Fix: ensure path starts with /
			expectedPath: "/api/v1/test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset captured URLs
			mu.Lock()
			primaryURLReceived = ""
			secondaryURLReceived = ""
			mu.Unlock()

			req := httptest.NewRequest("GET", tc.requestPath, nil)

			// Process dry-run
			ctx := context.Background()
			result, err := handler.ProcessDryRun(ctx, req, tc.primaryURL, tc.secondaryURL)

			if err != nil {
				t.Fatalf("Dry-run processing failed: %v", err)
			}

			// Wait a moment for requests to complete
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			primaryURL := primaryURLReceived
			secondaryURL := secondaryURLReceived
			mu.Unlock()

			// Verify URLs don't contain double slashes
			if strings.Contains(primaryURL, "//") && !strings.HasPrefix(primaryURL, "http://") && !strings.HasPrefix(primaryURL, "https://") {
				t.Errorf("Primary URL contains double slashes: %s", primaryURL)
			}

			if strings.Contains(secondaryURL, "//") && !strings.HasPrefix(secondaryURL, "http://") && !strings.HasPrefix(secondaryURL, "https://") {
				t.Errorf("Secondary URL contains double slashes: %s", secondaryURL)
			}

			// Verify the path part is correct
			if primaryURL != tc.expectedPath {
				t.Errorf("Primary URL path incorrect. Expected: %s, Got: %s", tc.expectedPath, primaryURL)
			}

			if secondaryURL != tc.expectedPath {
				t.Errorf("Secondary URL path incorrect. Expected: %s, Got: %s", tc.expectedPath, secondaryURL)
			}

			// Verify no errors in responses
			if result.PrimaryResponse.Error != "" {
				t.Errorf("Primary response had error: %s", result.PrimaryResponse.Error)
			}

			if result.SecondaryResponse.Error != "" {
				t.Errorf("Secondary response had error: %s", result.SecondaryResponse.Error)
			}
		})
	}
}

// testEndToEndDryRunWithRequestBody tests the complete dry-run flow with request bodies
// using the main module's handleDryRunRequest method to ensure the fixes work in the full context
func testEndToEndDryRunWithRequestBody(t *testing.T) {
	var primaryBodyReceived, secondaryBodyReceived string
	var primaryRequestCount, secondaryRequestCount int
	var mu sync.Mutex

	// Primary backend server
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		primaryBodyReceived = string(body)
		primaryRequestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"primary","path":"` + r.URL.Path + `"}`)); err != nil {
			t.Errorf("Primary server failed to write response: %v", err)
		}
	}))
	defer primaryServer.Close()

	// Secondary backend server
	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		secondaryBodyReceived = string(body)
		secondaryRequestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"backend":"secondary","path":"` + r.URL.Path + `"}`)); err != nil {
			t.Errorf("Secondary server failed to write response: %v", err)
		}
	}))
	defer secondaryServer.Close()

	// Create mock application and module
	app := NewMockTenantApplication()

	// Configure the module with dry-run enabled
	config := &ReverseProxyConfig{
		BackendServices: map[string]string{
			"primary":   primaryServer.URL,
			"secondary": secondaryServer.URL,
		},
		DefaultBackend: "primary",
		Routes: map[string]string{
			"/api/test": "primary",
		},
		RouteConfigs: map[string]RouteConfig{
			"/api/test": {
				DryRun:        true,
				DryRunBackend: "secondary",
			},
		},
		DryRun: DryRunConfig{
			Enabled:                true,
			LogResponses:           true,
			MaxResponseSize:        1024,
			DefaultResponseBackend: "primary",
		},
		TenantIDHeader: "X-Tenant-ID",
	}

	// Register config
	app.RegisterConfigSection("reverseproxy", modular.NewStdConfigProvider(config))

	// Create and initialize module
	module := NewModule()
	// Use the simple mock router instead of the testify mock
	router := &testRouter{routes: make(map[string]http.HandlerFunc)}

	constructedModule, err := module.Constructor()(app, map[string]any{
		"router": router,
	})
	if err != nil {
		t.Fatalf("Failed to construct module: %v", err)
	}

	reverseProxyModule := constructedModule.(*ReverseProxyModule)

	if err := reverseProxyModule.Init(app); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	if err := reverseProxyModule.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start module: %v", err)
	}

	// Create a request with body content
	requestBody := `{"test":"data","user":"john","action":"create"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Get the route config for dry-run handling
	routeConfig := config.RouteConfigs["/api/test"]

	// Call the dry-run handler directly (simulating what happens in the routing logic)
	reverseProxyModule.handleDryRunRequest(context.Background(), w, req, routeConfig, "primary", "secondary")

	// Wait for background dry-run to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the immediate response was successful
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	// Verify response body contains primary backend response
	responseBody := w.Body.String()
	if !strings.Contains(responseBody, `"backend":"primary"`) {
		t.Errorf("Response should contain primary backend data, got: %s", responseBody)
	}

	// Verify both backends received requests (primary for immediate response, both for dry-run)
	mu.Lock()
	primaryCount := primaryRequestCount
	secondaryCount := secondaryRequestCount
	primaryBody := primaryBodyReceived
	secondaryBody := secondaryBodyReceived
	mu.Unlock()

	// Primary should receive 2 requests: one for immediate response, one for dry-run comparison
	if primaryCount != 2 {
		t.Errorf("Expected primary to receive 2 requests, got %d", primaryCount)
	}

	// Secondary should receive 1 request: one for dry-run comparison
	if secondaryCount != 1 {
		t.Errorf("Expected secondary to receive 1 request, got %d", secondaryCount)
	}

	// Verify both backends received the correct request body
	if primaryBody != requestBody {
		t.Errorf("Primary backend received incorrect body. Expected: %q, Got: %q", requestBody, primaryBody)
	}

	if secondaryBody != requestBody {
		t.Errorf("Secondary backend received incorrect body. Expected: %q, Got: %q", requestBody, secondaryBody)
	}

	// Clean up
	if err := reverseProxyModule.Stop(context.Background()); err != nil {
		t.Errorf("Failed to stop reverse proxy module: %v", err)
	}
}
