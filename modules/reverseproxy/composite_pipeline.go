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
)

const (
	// StrategyPipeline executes backends sequentially where each stage's response
	// can inform the next stage's request. This enables map/reduce patterns where
	// backend B's request is constructed from backend A's response.
	//
	// Example: Backend A returns a list of conversation IDs, backend B is called
	// with those IDs to fetch ancillary details, and the responses are merged.
	//
	// Requires a PipelineConfig to be set via SetPipelineConfig.
	StrategyPipeline CompositeStrategy = "pipeline"

	// StrategyFanOutMerge executes all backend requests in parallel (like merge),
	// then applies a custom FanOutMerger function to perform ID-based matching,
	// filtering, and complex merging logic across all responses.
	//
	// Example: Backend A returns conversations, backend B returns follow-up flags.
	// The merger matches by conversation ID and produces a unified response.
	//
	// Requires a FanOutMerger to be set via SetFanOutMerger.
	StrategyFanOutMerge CompositeStrategy = "fan-out-merge"
)

// EmptyResponsePolicy defines how empty backend responses should be handled
// in pipeline and fan-out-merge strategies.
type EmptyResponsePolicy string

const (
	// EmptyResponseAllow includes empty responses in the result set.
	// Backends that return no data are represented as empty/nil in the response map.
	EmptyResponseAllow EmptyResponsePolicy = "allow-empty"

	// EmptyResponseSkip silently drops empty responses from the result set.
	// The merger/pipeline receives only non-empty responses.
	EmptyResponseSkip EmptyResponsePolicy = "skip-empty"

	// EmptyResponseFail causes the entire composite request to fail if any backend
	// returns an empty response. Returns 502 Bad Gateway.
	EmptyResponseFail EmptyResponsePolicy = "fail-on-empty"
)

// PipelineRequestBuilder builds the HTTP request for the next pipeline stage.
// It receives:
//   - ctx: the request context
//   - originalReq: the original incoming HTTP request
//   - previousResponses: accumulated parsed response bodies keyed by backend ID
//   - nextBackendID: the ID of the next backend to call
//
// It returns the HTTP request to send to the next backend, or an error.
// If it returns nil for the request (with no error), the stage is skipped.
type PipelineRequestBuilder func(
	ctx context.Context,
	originalReq *http.Request,
	previousResponses map[string][]byte,
	nextBackendID string,
) (*http.Request, error)

// PipelineResponseMerger merges all pipeline stage responses into a single HTTP response.
// It receives:
//   - ctx: the request context
//   - originalReq: the original incoming HTTP request
//   - allResponses: all accumulated response bodies keyed by backend ID
//
// It returns the final merged HTTP response, or an error.
type PipelineResponseMerger func(
	ctx context.Context,
	originalReq *http.Request,
	allResponses map[string][]byte,
) (*http.Response, error)

// FanOutMerger merges parallel backend responses using custom logic such as
// ID-based matching, filtering, or complex data correlation.
// It receives:
//   - ctx: the request context
//   - originalReq: the original incoming HTTP request
//   - responses: response bodies keyed by backend ID
//
// It returns the final merged HTTP response, or an error.
type FanOutMerger func(
	ctx context.Context,
	originalReq *http.Request,
	responses map[string][]byte,
) (*http.Response, error)

// PipelineConfig holds configuration for a pipeline strategy route.
type PipelineConfig struct {
	// RequestBuilder constructs the request for each subsequent pipeline stage
	// using responses from previous stages.
	RequestBuilder PipelineRequestBuilder

	// ResponseMerger combines all pipeline stage responses into a final response.
	// If nil, a default merger is used that wraps all responses in a JSON object
	// keyed by backend ID.
	ResponseMerger PipelineResponseMerger
}

// isEmptyBody returns true if the body bytes represent an empty or null response.
func isEmptyBody(body []byte) bool {
	if len(body) == 0 {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return true
	}
	// Check for JSON null
	if string(trimmed) == "null" {
		return true
	}
	// Check for empty JSON object
	if string(trimmed) == "{}" {
		return true
	}
	// Check for empty JSON array
	if string(trimmed) == "[]" {
		return true
	}
	return false
}

// executePipeline executes backends sequentially, passing each response to the
// PipelineRequestBuilder to construct the next request.
func (h *CompositeHandler) executePipeline(ctx context.Context, w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	if h.pipelineConfig == nil || h.pipelineConfig.RequestBuilder == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Pipeline strategy requires a PipelineConfig with RequestBuilder"))
		return
	}

	allResponses := make(map[string][]byte)

	for i, backend := range h.backends {
		// Check the circuit breaker before making the request.
		circuitBreaker := h.circuitBreakers[backend.ID]
		if circuitBreaker != nil && circuitBreaker.IsOpen() {
			continue
		}

		var req *http.Request
		var err error

		if i == 0 {
			// First stage: use the original request
			req, err = h.buildBackendRequest(ctx, backend, r, bodyBytes)
			if err != nil {
				if circuitBreaker != nil {
					circuitBreaker.RecordFailure()
				}
				continue
			}
		} else {
			// Subsequent stages: use the PipelineRequestBuilder
			req, err = h.pipelineConfig.RequestBuilder(ctx, r, allResponses, backend.ID)
			if err != nil {
				if circuitBreaker != nil {
					circuitBreaker.RecordFailure()
				}
				continue
			}
			// If builder returns nil, skip this stage
			if req == nil {
				continue
			}
		}

		// Execute the request using the backend's client
		resp, err := backend.Client.Do(req) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends
		if err != nil {
			if circuitBreaker != nil {
				circuitBreaker.RecordFailure()
			}
			continue
		}

		// Read and store the response body
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
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

		// Apply empty response policy
		if isEmptyBody(respBody) {
			switch h.emptyResponsePolicy {
			case EmptyResponseFail:
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprintf(w, "Backend %s returned empty response", backend.ID)
				return
			case EmptyResponseSkip:
				continue
			case EmptyResponseAllow:
				// Include empty response
			default:
				// Include empty response
			}
		}

		allResponses[backend.ID] = respBody
	}

	// Merge all responses
	if h.pipelineConfig.ResponseMerger != nil {
		mergedResp, err := h.pipelineConfig.ResponseMerger(ctx, r, allResponses)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Pipeline response merge failed: %v", err)
			return
		}
		if mergedResp != nil {
			h.writeResponse(mergedResp, w)
			mergedResp.Body.Close()
			return
		}
	}

	// Default: wrap all responses in a JSON object keyed by backend ID
	h.writeDefaultPipelineResponse(allResponses, w)
}

// executeFanOutMerge executes all backend requests in parallel, reads their bodies,
// then applies the FanOutMerger to produce the final response.
func (h *CompositeHandler) executeFanOutMerge(ctx context.Context, w http.ResponseWriter, r *http.Request, bodyBytes []byte) {
	if h.fanOutMerger == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Fan-out-merge strategy requires a FanOutMerger"))
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	responses := make(map[string][]byte)

	for _, backend := range h.backends {
		b := backend
		wg.Go(func() {
			// Check the circuit breaker
			circuitBreaker := h.circuitBreakers[b.ID]
			if circuitBreaker != nil && circuitBreaker.IsOpen() {
				return
			}

			// Execute the request
			resp, err := h.executeBackendRequest(ctx, b, r, bodyBytes) //nolint:bodyclose // Response body is closed below
			if err != nil {
				if circuitBreaker != nil {
					circuitBreaker.RecordFailure()
				}
				return
			}

			// Read the response body
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				if circuitBreaker != nil {
					circuitBreaker.RecordFailure()
				}
				return
			}

			// Record success
			if circuitBreaker != nil {
				circuitBreaker.RecordSuccess()
			}

			mu.Lock()
			responses[b.ID] = body
			mu.Unlock()
		})
	}

	wg.Wait()

	// Short-circuit if all backends failed or were skipped by open circuit breakers
	if len(responses) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "No successful responses from fan-out backends")
		return
	}

	// Apply empty response policy
	filteredResponses := make(map[string][]byte)
	for backendID, body := range responses {
		if isEmptyBody(body) {
			switch h.emptyResponsePolicy {
			case EmptyResponseFail:
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprintf(w, "Backend %s returned empty response", backendID)
				return
			case EmptyResponseSkip:
				continue
			case EmptyResponseAllow:
				filteredResponses[backendID] = body
			default:
				filteredResponses[backendID] = body
			}
		} else {
			filteredResponses[backendID] = body
		}
	}

	// Short-circuit if all responses were filtered out (e.g., all empty with skip policy)
	if len(filteredResponses) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "No non-empty responses from fan-out backends")
		return
	}

	// Apply the fan-out merger
	mergedResp, err := h.fanOutMerger(ctx, r, filteredResponses)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Fan-out merge failed: %v", err)
		return
	}
	if mergedResp != nil {
		h.writeResponse(mergedResp, w)
		mergedResp.Body.Close()
		return
	}

	// If merger returned nil, return empty response
	w.WriteHeader(http.StatusNoContent)
}

// buildBackendRequest creates an HTTP request for a backend (used by pipeline for the first stage).
func (h *CompositeHandler) buildBackendRequest(ctx context.Context, backend *Backend, r *http.Request, bodyBytes []byte) (*http.Request, error) {
	backendURL := backend.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, backendURL, nil) //nolint:gosec // G704: reverse proxy intentionally forwards requests to configured backends
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range r.Header {
		for _, val := range v {
			req.Header.Add(k, val)
		}
	}

	if len(bodyBytes) > 0 {
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))
	}

	return req, nil
}

// writeDefaultPipelineResponse writes a default JSON response containing all pipeline stage responses.
func (h *CompositeHandler) writeDefaultPipelineResponse(allResponses map[string][]byte, w http.ResponseWriter) {
	if len(allResponses) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("No successful responses from pipeline backends"))
		return
	}

	merged := make(map[string]interface{})
	for backendID, body := range allResponses {
		var data interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			merged[backendID] = string(body)
		} else {
			merged[backendID] = data
		}
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to encode pipeline response"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

// MakeJSONResponse is a helper that creates an HTTP response from a JSON-serializable value.
// It's provided for use by PipelineResponseMerger and FanOutMerger implementations.
func MakeJSONResponse(statusCode int, data interface{}) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}
