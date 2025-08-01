// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"net/http/httptest"
)

// Backend represents a backend service configuration.
type Backend struct {
	ID     string
	URL    string
	Client *http.Client
}

// CompositeHandler is updated to handle multiple requests and process/merge them
// into a single response. It now includes circuit breaking and response caching.
type CompositeHandler struct {
	backends        []*Backend
	parallel        bool // Flag to control parallel execution of requests
	responseTimeout time.Duration
	circuitBreakers map[string]*CircuitBreaker
	responseCache   *responseCache
}

// NewCompositeHandler creates a new composite handler with the given backends.
func NewCompositeHandler(backends []*Backend, parallel bool, responseTimeout time.Duration) *CompositeHandler {
	// Initialize circuit breakers for each backend - using default settings
	// These will be replaced when ConfigureCircuitBreakers is called
	circuitBreakers := make(map[string]*CircuitBreaker)
	for _, b := range backends {
		circuitBreakers[b.ID] = nil
	}

	return &CompositeHandler{
		backends:        backends,
		parallel:        parallel,
		responseTimeout: responseTimeout,
		circuitBreakers: circuitBreakers,
		// No caching by default, can be set via SetResponseCache.
	}
}

// ConfigureCircuitBreakers sets up circuit breakers for each backend using the provided configuration
func (h *CompositeHandler) ConfigureCircuitBreakers(globalConfig CircuitBreakerConfig, backendConfigs map[string]CircuitBreakerConfig) {
	for _, backend := range h.backends {
		// Check if there's a backend-specific configuration
		if backendConfig, exists := backendConfigs[backend.ID]; exists {
			// Use backend-specific configuration if it exists
			if backendConfig.Enabled {
				h.circuitBreakers[backend.ID] = NewCircuitBreaker(backend.ID, nil)
			} else {
				// Circuit breaker is explicitly disabled for this backend
				h.circuitBreakers[backend.ID] = nil
			}
		} else if globalConfig.Enabled {
			// Use global configuration
			h.circuitBreakers[backend.ID] = NewCircuitBreaker(backend.ID, nil)
		} else {
			// Circuit breaker is disabled globally
			h.circuitBreakers[backend.ID] = nil
		}
	}
}

// SetResponseCache sets a response cache for the handler.
func (h *CompositeHandler) SetResponseCache(cache *responseCache) {
	h.responseCache = cache
}

// ServeHTTP handles the request by forwarding it to all backends
// and merging the responses.
func (h *CompositeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to get response from cache first if caching is enabled.
	if h.responseCache != nil && r.Method == http.MethodGet {
		cacheKey := h.responseCache.GenerateKey(r)
		if cachedResp, found := h.responseCache.Get(cacheKey); found {
			// Return cached response.
			for k, v := range cachedResp.Headers {
				for _, val := range v {
					w.Header().Add(k, val)
				}
			}
			w.WriteHeader(cachedResp.StatusCode)
			if _, err := w.Write(cachedResp.Body); err != nil {
				http.Error(w, "Failed to write cached response", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	// Create a response recorder to capture the merged response.
	recorder := httptest.NewRecorder()

	// Create a context with timeout for all backend requests.
	ctx, cancel := context.WithTimeout(r.Context(), h.responseTimeout)
	defer cancel()

	// Use either parallel or sequential execution based on configuration.
	if h.parallel {
		h.executeParallel(ctx, recorder, r)
	} else {
		h.executeSequential(ctx, recorder, r)
	}

	// Get the final response from the recorder.
	resp := recorder.Result()

	// Cache the response if appropriate.
	if h.responseCache != nil && h.responseCache.IsCacheable(r, resp.StatusCode) {
		// Read the response body.
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			// Reset the body for later reading.
			resp.Body = io.NopCloser(bytes.NewBuffer(body))

			// Cache the response.
			cacheKey := h.responseCache.GenerateKey(r)
			h.responseCache.Set(cacheKey, resp.StatusCode, resp.Header, body, 0) // Use default TTL.
		}
	}

	// Copy headers to the response writer.
	for k, v := range resp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	// Write status code.
	w.WriteHeader(resp.StatusCode)

	// Copy body to the response writer.
	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, "Failed to write response body", http.StatusInternalServerError)
		return
	}
}

// executeParallel executes all backend requests in parallel.
func (h *CompositeHandler) executeParallel(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	responses := make(map[string]*http.Response)

	// Create a wait group to track each backend request.
	for _, backend := range h.backends {
		wg.Add(1)

		// Execute each request in a separate goroutine.
		go func(b *Backend) {
			defer wg.Done()

			// Check the circuit breaker before making the request.
			circuitBreaker := h.circuitBreakers[b.ID]
			if circuitBreaker != nil && circuitBreaker.IsOpen() {
				// Circuit is open, skip this backend.
				return
			}

			// Execute the request.
			resp, err := h.executeBackendRequest(ctx, b, r) //nolint:bodyclose // Response body is closed in mergeResponses cleanup
			if err != nil {
				if circuitBreaker != nil {
					circuitBreaker.RecordFailure()
				}
				return
			}

			// Record success in the circuit breaker.
			if circuitBreaker != nil {
				circuitBreaker.RecordSuccess()
			}

			// Store the response.
			mu.Lock()
			responses[b.ID] = resp
			mu.Unlock()
		}(backend)
	}

	// Wait for all requests to complete.
	wg.Wait()

	// Merge the responses.
	h.mergeResponses(responses, w)

	// Close all response bodies to prevent resource leaks
	for _, resp := range responses {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
}

// executeSequential executes backend requests one at a time.
func (h *CompositeHandler) executeSequential(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	responses := make(map[string]*http.Response)

	// Execute each request sequentially.
	for _, backend := range h.backends {
		// Check the circuit breaker before making the request.
		circuitBreaker := h.circuitBreakers[backend.ID]
		if circuitBreaker != nil && circuitBreaker.IsOpen() {
			// Circuit is open, skip this backend.
			continue
		}

		// Execute the request.
		resp, err := h.executeBackendRequest(ctx, backend, r) //nolint:bodyclose // Response body is closed in mergeResponses cleanup
		if err != nil {
			if circuitBreaker != nil {
				circuitBreaker.RecordFailure()
			}
			continue
		}

		// Record success in the circuit breaker.
		if circuitBreaker != nil {
			circuitBreaker.RecordSuccess()
		}

		// Store the response.
		responses[backend.ID] = resp
	}

	// Merge the responses.
	h.mergeResponses(responses, w)

	// Close all response bodies to prevent resource leaks
	for _, resp := range responses {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
}

// executeBackendRequest sends a request to a backend and returns the response.
func (h *CompositeHandler) executeBackendRequest(ctx context.Context, backend *Backend, r *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original.
	backendURL := backend.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	// Create a new request with the same method, URL, and headers.
	req, err := http.NewRequestWithContext(ctx, r.Method, backendURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Copy all headers from the original request.
	for k, v := range r.Header {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	// Properly handle the request body if present.
	if r.Body != nil {
		// Get the body content.
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}

		// Reset the original request body so it can be read again.
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Set the body for the new request.
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Set content length.
		req.ContentLength = int64(len(bodyBytes))
	}

	// Execute the request.
	resp, err := backend.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute backend request: %w", err)
	}
	return resp, nil
}

// mergeResponses merges the responses from all backends.
func (h *CompositeHandler) mergeResponses(responses map[string]*http.Response, w http.ResponseWriter) {
	// If no responses, return 502 Bad Gateway.
	if len(responses) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte("No successful responses from backends"))
		if err != nil {
			// Log error but continue processing
			return
		}
		return
	}

	// Find the first available response based on the original backend order
	var baseResp *http.Response
	for _, backend := range h.backends {
		if resp, ok := responses[backend.ID]; ok {
			baseResp = resp
			break
		}
	}

	// If no response found based on backend order, fall back to any response
	if baseResp == nil {
		for _, resp := range responses {
			baseResp = resp
			break
		}
	}

	// Make sure baseResp is not nil before processing
	if baseResp == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte("Failed to process backend responses"))
		if err != nil {
			// Log error but continue processing
			return
		}
		return
	}

	// Copy headers from the base response.
	for k, v := range baseResp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	// Write the status code from the base response.
	w.WriteHeader(baseResp.StatusCode)

	// Copy the body from the base response.
	_, err := io.Copy(w, baseResp.Body)
	if err != nil {
		// Log error but continue processing
		return
	}
}

// createCompositeHandler creates a handler for a composite route configuration.
// It returns a handler that fetches responses from multiple backends and combines them.
// If tenantConfig is provided, it uses that config for backend URLs, otherwise falls back to global config.
func (m *ReverseProxyModule) createCompositeHandler(routeConfig CompositeRoute, tenantConfig *ReverseProxyConfig) (*CompositeHandler, error) {
	var backends []*Backend

	// Default response timeout if not specified in config
	responseTimeout := 30 * time.Second

	for _, backendName := range routeConfig.Backends {
		var backendURL string
		// Try to get backend URL from tenant config first
		if tenantConfig != nil && tenantConfig.BackendServices != nil {
			if url, ok := tenantConfig.BackendServices[backendName]; ok {
				backendURL = url
			}
		}

		// Fall back to global config if tenant config doesn't have this backend
		if backendURL == "" {
			if url, ok := m.config.BackendServices[backendName]; ok {
				backendURL = url
			} else {
				return nil, fmt.Errorf("%w: %s", ErrBackendServiceNotFound, backendName)
			}
		}

		// Add to backends list
		backends = append(backends, &Backend{
			ID:     backendName,
			URL:    backendURL,
			Client: m.httpClient, // Use the module's HTTP client directly
		})
	}

	// Create and configure the handler
	handler := NewCompositeHandler(backends, true, responseTimeout)

	// Configure circuit breakers
	if m.circuitBreakers != nil {
		for backendID, cb := range m.circuitBreakers {
			handler.circuitBreakers[backendID] = cb
		}
	}

	// Set response cache if available
	if m.responseCache != nil {
		handler.SetResponseCache(m.responseCache)
	}

	return handler, nil
}

// createFeatureFlagAwareCompositeHandlerFunc creates a http.HandlerFunc that evaluates feature flags
// before delegating to the composite handler.
func (m *ReverseProxyModule) createFeatureFlagAwareCompositeHandlerFunc(routeConfig CompositeRoute, tenantConfig *ReverseProxyConfig) (http.HandlerFunc, error) {
	// Create the underlying composite handler
	compositeHandler, err := m.createCompositeHandler(routeConfig, tenantConfig)
	if err != nil {
		return nil, err
	}

	// Return a wrapper function that checks feature flags
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this composite route is controlled by a feature flag
		if routeConfig.FeatureFlagID != "" && !m.evaluateFeatureFlag(routeConfig.FeatureFlagID, r) {
			// Feature flag is disabled, use alternative backend if available
			alternativeBackend := m.getAlternativeBackend(routeConfig.AlternativeBackend)
			if alternativeBackend != "" {
				// Route to alternative backend instead of composite route
				m.app.Logger().Debug("Composite route feature flag disabled, using alternative backend",
					"route", routeConfig.Pattern, "alternative", alternativeBackend, "flagID", routeConfig.FeatureFlagID)

				// Create a simple proxy handler for the alternative backend
				altHandler := m.createBackendProxyHandler(alternativeBackend)
				altHandler(w, r)
				return
			} else {
				// No alternative, return 404
				m.app.Logger().Debug("Composite route feature flag disabled, no alternative available",
					"route", routeConfig.Pattern, "flagID", routeConfig.FeatureFlagID)
				http.NotFound(w, r)
				return
			}
		}

		// Feature flag is enabled or not specified, proceed with composite logic
		compositeHandler.ServeHTTP(w, r)
	}, nil
}
