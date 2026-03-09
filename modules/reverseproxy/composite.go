// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"net/http/httptest"
)

// CompositeStrategy defines how responses from multiple backends are combined.
type CompositeStrategy string

const (
	// StrategyFirstSuccess returns the first successful response from backends (in order).
	// Backends are tried sequentially until one succeeds.
	StrategyFirstSuccess CompositeStrategy = "first-success"

	// StrategyMerge attempts to merge JSON responses from all backends into a single JSON object.
	// Requests are executed in parallel and responses are combined.
	StrategyMerge CompositeStrategy = "merge"

	// StrategySequential executes requests sequentially and returns the last successful response.
	// This is useful when later backends depend on earlier ones completing.
	StrategySequential CompositeStrategy = "sequential"
)

// ResponseTransformer is a function that can transform backend responses.
// It receives a map of backend responses (keyed by backend ID) and can modify them
// or create a new combined response. This allows for complex response manipulation
// like merging specific fields, data augmentation, etc.
type ResponseTransformer func(responses map[string]*http.Response) (*http.Response, error)

// Backend represents a backend service configuration.
type Backend struct {
	ID     string
	URL    string
	Client *http.Client
}

// CompositeHandler is updated to handle multiple requests and process/merge them
// into a single response. It now includes circuit breaking and response caching.
type CompositeHandler struct {
	backends            []*Backend
	strategy            CompositeStrategy
	responseTimeout     time.Duration
	circuitBreakers     map[string]*CircuitBreaker
	responseCache       *responseCache
	eventEmitter        func(eventType string, data map[string]interface{})
	responseTransformer ResponseTransformer
}

// NewCompositeHandler creates a new composite handler with the given backends and strategy.
func NewCompositeHandler(backends []*Backend, strategy CompositeStrategy, responseTimeout time.Duration) *CompositeHandler {
	// Initialize circuit breakers for each backend - using default settings
	// These will be replaced when ConfigureCircuitBreakers is called
	circuitBreakers := make(map[string]*CircuitBreaker)
	for _, b := range backends {
		circuitBreakers[b.ID] = nil
	}

	// Default to first-success if no strategy specified
	if strategy == "" {
		strategy = StrategyFirstSuccess
	}

	return &CompositeHandler{
		backends:        backends,
		strategy:        strategy,
		responseTimeout: responseTimeout,
		circuitBreakers: circuitBreakers,
		// No caching by default, can be set via SetResponseCache.
	}
}

// SetEventEmitter sets the event emitter function for the composite handler.
func (h *CompositeHandler) SetEventEmitter(emitter func(eventType string, data map[string]interface{})) {
	h.eventEmitter = emitter
}

// ConfigureCircuitBreakers sets up circuit breakers for each backend using the provided configuration
func (h *CompositeHandler) ConfigureCircuitBreakers(globalConfig CircuitBreakerConfig, backendConfigs map[string]CircuitBreakerConfig) {
	for _, backend := range h.backends {
		// Check if there's a backend-specific configuration
		if backendConfig, exists := backendConfigs[backend.ID]; exists {
			// Use backend-specific configuration if it exists
			if backendConfig.Enabled {
				cb := NewCircuitBreakerWithConfig(backend.ID, backendConfig, nil)
				cb.eventEmitter = h.eventEmitter
				h.circuitBreakers[backend.ID] = cb
			} else {
				// Circuit breaker is explicitly disabled for this backend
				h.circuitBreakers[backend.ID] = nil
			}
		} else if globalConfig.Enabled {
			// Use global configuration
			cb := NewCircuitBreakerWithConfig(backend.ID, globalConfig, nil)
			cb.eventEmitter = h.eventEmitter
			h.circuitBreakers[backend.ID] = cb
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

// SetResponseTransformer sets a custom response transformer function.
func (h *CompositeHandler) SetResponseTransformer(transformer ResponseTransformer) {
	h.responseTransformer = transformer
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
			if _, err := w.Write(cachedResp.Body); err != nil { //nolint:gosec // G705: reverse proxy transparently forwards upstream response body
				http.Error(w, "Failed to write cached response", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	// Create a response recorder to capture the merged response.
	recorder := httptest.NewRecorder()

	// Read and buffer the request body once (if any) before launching parallel goroutines.
	var bodyBytes []byte
	if r.Body != nil {
		// ReadAll returns empty slice and nil error for empty body; that's fine.
		if data, err := io.ReadAll(r.Body); err == nil {
			bodyBytes = data
			// Reset original request body so downstream middleware (if any) can still read it later.
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		} else {
			// On error we log by returning an error response; safer than racing later.
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
	}

	// Create a context with timeout for all backend requests.
	ctx, cancel := context.WithTimeout(r.Context(), h.responseTimeout)
	defer cancel()

	// Execute requests based on strategy
	switch h.strategy {
	case StrategyFirstSuccess:
		h.executeFirstSuccess(ctx, recorder, r, bodyBytes)
	case StrategyMerge:
		h.executeMerge(ctx, recorder, r, bodyBytes)
	case StrategySequential:
		h.executeSequential(ctx, recorder, r, bodyBytes)
	default:
		// Default to first-success for unknown strategies
		h.executeFirstSuccess(ctx, recorder, r, bodyBytes)
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

// executeFirstSuccess tries backends sequentially until one succeeds, returning the first successful response.
func (h *CompositeHandler) executeFirstSuccess(ctx context.Context, w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	// Try each backend in order until one succeeds
	for _, backend := range h.backends {
		// Check the circuit breaker before making the request.
		circuitBreaker := h.circuitBreakers[backend.ID]
		if circuitBreaker != nil && circuitBreaker.IsOpen() {
			// Circuit is open, skip this backend.
			continue
		}

		// Execute the request.
		resp, err := h.executeBackendRequest(ctx, backend, r, bodyBytes) //nolint:bodyclose // Response body is closed after writing
		if err != nil {
			if circuitBreaker != nil {
				circuitBreaker.RecordFailure()
			}
			continue
		}

		// Check if the response is successful (2xx or 3xx status code)
		if resp.StatusCode >= 400 {
			// Response has an error status code, try next backend
			resp.Body.Close()
			if circuitBreaker != nil {
				circuitBreaker.RecordFailure()
			}
			continue
		}

		// Record success in the circuit breaker.
		if circuitBreaker != nil {
			circuitBreaker.RecordSuccess()
		}

		// Found a successful response, write it and return
		h.writeResponse(resp, w)
		resp.Body.Close()
		return
	}

	// No successful responses
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte("No successful responses from backends"))
}

// executeMerge executes all backend requests in parallel and merges their responses.
func (h *CompositeHandler) executeMerge(ctx context.Context, w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	responses := make(map[string]*http.Response)

	// Create a wait group to track each backend request.
	for _, backend := range h.backends {
		b := backend // capture loop variable
		wg.Go(func() {
			// Check the circuit breaker before making the request.
			circuitBreaker := h.circuitBreakers[b.ID]
			if circuitBreaker != nil && circuitBreaker.IsOpen() {
				// Circuit is open, skip this backend.
				return
			}

			// Execute the request.
			resp, err := h.executeBackendRequest(ctx, b, r, bodyBytes) //nolint:bodyclose // Response body is closed in mergeResponses cleanup
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
		})
	}

	// Wait for all requests to complete.
	wg.Wait()

	// If custom transformer is set, use it
	if h.responseTransformer != nil {
		transformedResp, err := h.responseTransformer(responses)
		if err == nil && transformedResp != nil {
			h.writeResponse(transformedResp, w)
			transformedResp.Body.Close()
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Response transformation failed"))
		}
	} else {
		// Default merge behavior: merge JSON responses
		h.mergeJSONResponses(responses, w)
	}

	// Close all response bodies to prevent resource leaks
	for _, resp := range responses {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}
}

// executeSequential executes backend requests one at a time, returning the last successful response.
func (h *CompositeHandler) executeSequential(ctx context.Context, w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	var lastSuccessfulResp *http.Response

	// Execute each request sequentially.
	for _, backend := range h.backends {
		// Check the circuit breaker before making the request.
		circuitBreaker := h.circuitBreakers[backend.ID]
		if circuitBreaker != nil && circuitBreaker.IsOpen() {
			// Circuit is open, skip this backend.
			continue
		}

		// Execute the request.
		resp, err := h.executeBackendRequest(ctx, backend, r, bodyBytes) //nolint:bodyclose // Response body is closed after use
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

		// Close previous response if any
		if lastSuccessfulResp != nil && lastSuccessfulResp.Body != nil {
			lastSuccessfulResp.Body.Close()
		}

		// Store this as the last successful response
		lastSuccessfulResp = resp
	}

	// Write the last successful response
	if lastSuccessfulResp != nil {
		h.writeResponse(lastSuccessfulResp, w)
		lastSuccessfulResp.Body.Close()
	} else {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("No successful responses from backends"))
	}
}

// executeBackendRequest sends a request to a backend and returns the response.
func (h *CompositeHandler) executeBackendRequest(ctx context.Context, backend *Backend, r *http.Request, bodyBytes []byte) (*http.Response, error) {
	// Clone the request to avoid modifying the original.
	backendURL := backend.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	// Create a new request with the same method, URL, and headers.
	req, err := http.NewRequestWithContext(ctx, r.Method, backendURL, nil) //nolint:gosec // G704: backendURL is built from configured backend.URL, not user input
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Copy all headers from the original request.
	for k, v := range r.Header {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	// Attach pre-read body (if any) without mutating the shared request.
	if len(bodyBytes) > 0 {
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
	}

	// Execute the request.
	resp, err := backend.Client.Do(req) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends
	if err != nil {
		return nil, fmt.Errorf("failed to execute backend request: %w", err)
	}
	return resp, nil
}

// writeResponse writes a single response to the response writer.
func (h *CompositeHandler) writeResponse(resp *http.Response, w http.ResponseWriter) {
	// Copy headers from the response
	for k, v := range resp.Header {
		for _, val := range v {
			w.Header().Add(k, val)
		}
	}

	// Write the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the body
	_, _ = io.Copy(w, resp.Body)
}

// mergeJSONResponses merges JSON responses from all backends into a single JSON object.
func (h *CompositeHandler) mergeJSONResponses(responses map[string]*http.Response, w http.ResponseWriter) {
	// If no responses, return 502 Bad Gateway.
	if len(responses) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("No successful responses from backends"))
		return
	}

	// Merged JSON object
	merged := make(map[string]interface{})

	// Parse each response as JSON and add to merged object
	for backendID, resp := range responses {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			// If not JSON, store as raw string
			merged[backendID] = string(body)
		} else {
			merged[backendID] = data
		}
	}

	// Write merged JSON
	encoded, err := json.Marshal(merged)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to encode merged response"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

// createCompositeHandler creates a handler for a composite route configuration.
// It returns a handler that fetches responses from multiple backends and combines them.
// If tenantConfig is provided, it uses that config for backend URLs, otherwise falls back to global config.
func (m *ReverseProxyModule) createCompositeHandler(ctx context.Context, routeConfig CompositeRoute, tenantConfig *ReverseProxyConfig) (*CompositeHandler, error) {
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

	// Determine the strategy to use
	strategy := CompositeStrategy(routeConfig.Strategy)
	if strategy == "" {
		strategy = StrategyFirstSuccess // default
	}

	// Create and configure the handler
	handler := NewCompositeHandler(backends, strategy, responseTimeout)

	// Set event emitter for circuit breaker events
	handler.SetEventEmitter(func(eventType string, data map[string]interface{}) {
		m.emitEvent(ctx, eventType, data)
	})

	// Configure circuit breakers using the module's configuration
	if m.config != nil {
		// Use tenant config if available, otherwise use global config
		config := m.config
		if tenantConfig != nil {
			config = tenantConfig
		}

		globalCBConfig := config.CircuitBreakerConfig
		backendCBConfigs := make(map[string]CircuitBreakerConfig)
		if config.BackendCircuitBreakers != nil {
			backendCBConfigs = config.BackendCircuitBreakers
		}

		handler.ConfigureCircuitBreakers(globalCBConfig, backendCBConfigs)
	}

	// Set response cache if available
	if m.responseCache != nil {
		handler.SetResponseCache(m.responseCache)
	}

	// Set response transformer if available for this route
	if transformer, exists := m.responseTransformers[routeConfig.Pattern]; exists {
		handler.SetResponseTransformer(transformer)
	}

	return handler, nil
}

// createFeatureFlagAwareCompositeHandlerFunc creates a http.HandlerFunc that evaluates feature flags
// before delegating to the composite handler.
func (m *ReverseProxyModule) createFeatureFlagAwareCompositeHandlerFunc(ctx context.Context, routeConfig CompositeRoute, tenantConfig *ReverseProxyConfig) (http.HandlerFunc, error) {
	// Create the underlying composite handler
	compositeHandler, err := m.createCompositeHandler(ctx, routeConfig, tenantConfig)
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
				// Check if dry-run mode is enabled for this scenario
				effectiveConfig := m.getEffectiveConfigForRequest(r)
				isDryRunEnabled := (effectiveConfig != nil && effectiveConfig.DryRun.Enabled)

				if isDryRunEnabled {
					// Use dry-run handler to compare composite vs alternative
					m.app.Logger().Debug("Feature flag disabled with dry-run enabled, comparing composite vs alternative",
						"route", routeConfig.Pattern, "alternative", alternativeBackend, "flagID", routeConfig.FeatureFlagID)

					// Create a mock RouteConfig for dry-run handling
					mockRouteConfig := RouteConfig{
						FeatureFlagID:      routeConfig.FeatureFlagID,
						AlternativeBackend: alternativeBackend,
						DryRun:             true,
						DryRunBackend:      "composite", // Compare against composite
					}

					// Handle dry-run comparison: alternative (returned) vs composite (compared)
					m.handleDryRunRequest(r.Context(), w, r, mockRouteConfig, alternativeBackend, "composite")
					return
				} else {
					// Route to alternative backend instead of composite route
					m.app.Logger().Debug("Composite route feature flag disabled, using alternative backend",
						"route", routeConfig.Pattern, "alternative", alternativeBackend, "flagID", routeConfig.FeatureFlagID)

					// Create a simple proxy handler for the alternative backend
					altHandler := m.createBackendProxyHandler(alternativeBackend)
					altHandler(w, r)
					return
				}
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
