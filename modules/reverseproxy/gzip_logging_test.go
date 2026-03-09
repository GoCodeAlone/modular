package reverseproxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noDecompressClient returns an *http.Client that never auto-decompresses
// responses, so test assertions see the raw wire bytes and headers.
func noDecompressClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableCompression: true},
	}
}

// gzipPayload compresses s and returns the raw gzip bytes.
func gzipPayload(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(s))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

// gzipBackend returns an httptest.Server whose handler always responds with
// Content-Encoding: gzip and the pre-compressed body.
func gzipBackend(body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
}

// TestReverseProxy_GzipPassthrough verifies that when the proxy's transport
// has DisableCompression: true (the mode used when an httpclient service with
// verbose logging is injected), the Content-Encoding header and compressed
// body pass through to the client intact.
//
// This is the configuration that triggers the httpclient loggingTransport's
// body-omission guard: it sees Content-Encoding: gzip on the response and
// writes "[body omitted: …]" instead of raw binary to the log.
func TestReverseProxy_GzipPassthrough(t *testing.T) {
	t.Parallel()

	const payload = `{"status":"ok","data":"hello gzip world"}`
	gzBody := gzipPayload(t, payload)

	backend := gzipBackend(gzBody)
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	require.NoError(t, err)

	// Build a reverse proxy the same way the module does when an httpclient
	// service is injected: transport with DisableCompression: true so
	// compressed responses pass through without Go auto-decompressing them.
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.Transport = &http.Transport{DisableCompression: true}

	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	// Use a client that also won't auto-decompress, so we can inspect the
	// raw response exactly as the proxy emitted it.
	client := noDecompressClient()
	resp, err := client.Get(frontend.URL + "/anything")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The proxy must preserve Content-Encoding so downstream logging
	// (httpclient loggingTransport) can detect gzip and omit the body.
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"),
		"Content-Encoding: gzip must be preserved through the proxy")

	// The raw body should still be the original gzip bytes.
	rawBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, gzBody, rawBody,
		"proxy must pass compressed bytes through unchanged")

	// Verify the payload decompresses correctly.
	gr, err := gzip.NewReader(bytes.NewReader(rawBody))
	require.NoError(t, err)
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, payload, string(decompressed),
		"decompressed payload must match original")
}

// TestReverseProxy_GzipDefaultTransportDecompresses verifies the contrasting
// behaviour: when DisableCompression is false (the module's default when no
// httpclient service is injected), Go's transport auto-decompresses the
// response body and strips the Content-Encoding header. In this mode, body
// logging sees plain text so no omission guard is needed.
func TestReverseProxy_GzipDefaultTransportDecompresses(t *testing.T) {
	t.Parallel()

	const payload = `{"status":"ok","data":"hello default transport"}`
	gzBody := gzipPayload(t, payload)

	backend := gzipBackend(gzBody)
	defer backend.Close()

	backendURL, err := url.Parse(backend.URL)
	require.NoError(t, err)

	// Default transport (DisableCompression: false) — Go auto-decompresses
	// responses when the transport itself added Accept-Encoding: gzip.
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.Transport = &http.Transport{DisableCompression: false}

	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	// Use a client that also has DisableCompression: false (the default) so
	// that the full chain behaves like production: the client's transport
	// adds Accept-Encoding: gzip, the proxy forwards it, and the client
	// auto-decompresses the proxied response.
	resp, err := http.Get(frontend.URL + "/anything")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// After auto-decompression, the body is plain text.
	assert.Equal(t, payload, string(body),
		"body should be the decompressed plain-text payload")
}
