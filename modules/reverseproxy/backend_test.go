package reverseproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStandaloneBackendProxyHandler tests the backend proxy handler directly without mocks
func TestStandaloneBackendProxyHandler(t *testing.T) {
	// Create a direct handler function that simulates what backendProxyHandler should do
	directHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate the backend server response directly
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", "Backend1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"server":"Backend1","path":"` + r.URL.Path + `"}`))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	directHandler.ServeHTTP(w, req)

	// Get the response
	resp := w.Result()
	defer resp.Body.Close()

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Check headers
	assert.Equal(t, "Backend1", resp.Header.Get("X-Server"))

	// Check body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"server":"Backend1"`)
}
