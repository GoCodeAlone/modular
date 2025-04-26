package reverseproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsolatedProxyBackend tests a simple proxy to a backend API server without using the module
func TestIsolatedProxyBackend(t *testing.T) {
	// Create a mock server for testing
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "Backend1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"server":"Backend1","path":"` + r.URL.Path + `"}`))
	}))
	defer mockServer.Close()

	// Create a handler that proxies to our mock server
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to the target server
		targetURL := mockServer.URL + r.URL.Path
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers from original request
		for k, vv := range r.Header {
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}

		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to perform request", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}

		// Copy status code
		w.WriteHeader(resp.StatusCode)

		// Copy body
		io.Copy(w, resp.Body)
	})

	// Test the handler
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	proxyHandler.ServeHTTP(w, req)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Verify status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify headers
	assert.Equal(t, "Backend1", resp.Header.Get("X-Server"))

	// Verify body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	require.NoError(t, err)

	assert.Equal(t, "Backend1", data["server"])
}

// TestIsolatedCompositeProxy tests a composite proxy to multiple API backends
func TestIsolatedCompositeProxy(t *testing.T) {
	// Create api1 mock server
	api1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "API1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"source":"api1","data":"api1 data"}`))
	}))
	defer api1Server.Close()

	// Create api2 mock server
	api2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "API2")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"source":"api2","data":"api2 data"}`))
	}))
	defer api2Server.Close()

	// Create Chi router for the test
	router := chi.NewRouter()

	// Create handler for the composite route
	compositeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create an HTTP client for outgoing requests
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		// Create and execute request to api1
		api1URL := api1Server.URL + r.URL.Path
		api1Req, _ := http.NewRequestWithContext(r.Context(), r.Method, api1URL, nil)
		api1Resp, api1Err := client.Do(api1Req)

		// Create and execute request to api2 API
		api2URL := api2Server.URL + r.URL.Path
		api2Req, _ := http.NewRequestWithContext(r.Context(), r.Method, api2URL, nil)
		api2Resp, api2Err := client.Do(api2Req)

		// Handle error cases
		if api1Err != nil && api2Err != nil {
			http.Error(w, "Failed to connect to both backends", http.StatusServiceUnavailable)
			return
		}

		// Create combined response
		result := map[string]interface{}{
			"combined": true,
		}

		// Include api1 data if available
		if api1Resp != nil {
			defer api1Resp.Body.Close()
			api1Body, _ := io.ReadAll(api1Resp.Body)
			var api1Data map[string]interface{}
			json.Unmarshal(api1Body, &api1Data)
			result["api1"] = api1Data
		}

		// Include api2 data if available
		if api2Resp != nil {
			defer api2Resp.Body.Close()
			api2Body, _ := io.ReadAll(api2Resp.Body)
			var api2Data map[string]interface{}
			json.Unmarshal(api2Body, &api2Data)
			result["api2"] = api2Data
		}

		// Send the combined response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	})

	// Register the test handler
	router.HandleFunc("/api/composite/test", compositeHandler)

	// Make a request to the composite endpoint
	req := httptest.NewRequest("GET", "/api/composite/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Check response code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	require.NoError(t, err)

	// Verify the structure
	assert.True(t, responseData["combined"].(bool))
	assert.NotNil(t, responseData["api1"])
	assert.NotNil(t, responseData["api2"])

	// Verify api1 data
	api1Data := responseData["api1"].(map[string]interface{})
	assert.Equal(t, "api1", api1Data["source"])
	assert.Equal(t, "api1 data", api1Data["data"])

	// Verify api2 data
	api2Data := responseData["api2"].(map[string]interface{})
	assert.Equal(t, "api2", api2Data["source"])
	assert.Equal(t, "api2 data", api2Data["data"])
}
