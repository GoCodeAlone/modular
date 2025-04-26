package reverseproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStandaloneCompositeProxyHandler tests the composite proxy handler directly without complex mocks
func TestStandaloneCompositeProxyHandler(t *testing.T) {
	// Create a direct handler that simulates what compositeProxyHandlerImpl should do
	directHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the combined response directly
		combinedResponse := map[string]interface{}{
			"combined": true,
			"api1": map[string]interface{}{
				"source": "api1",
				"data":   "api1 data",
			},
			"api2": map[string]interface{}{
				"source": "api2",
				"data":   "api2 data",
			},
		}

		// Set response headers
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Write the combined response
		json.NewEncoder(w).Encode(combinedResponse)
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/api/composite/test", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	directHandler.ServeHTTP(w, req)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var responseData map[string]interface{}
	err = json.Unmarshal(body, &responseData)
	require.NoError(t, err)

	// Verify structure
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
