package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/modules/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGzipBodyOmittedInLogsWhenProxied is an end-to-end integration test
// verifying that when a gzip-compressed response passes through a reverse
// proxy using the httpclient module's transport (with verbose logging), the
// log output contains the body-omission notice instead of raw binary bytes.
//
// This mirrors the real wiring: reverseproxy.module.go:1581 sets
// proxy.Transport = m.httpClient.Transport, so the httpclient's
// loggingTransport wraps every proxied request.
func TestGzipBodyOmittedInLogsWhenProxied(t *testing.T) {
	// 1. Create a gzip-compressed payload and a backend that serves it.
	const payload = `{"status":"ok","data":"hello gzip integration"}`
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, err := gw.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	gzBody := gzBuf.Bytes()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gzBody)
	}))
	defer backend.Close()

	// 2. Create a log-capturing buffer wired to a slog logger.
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 3. Bootstrap the modular app with the httpclient module.
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		logger,
	)

	// The httpclient module's RegisterConfig registers default values that
	// overwrite any pre-registered config section. Use OnConfigLoaded to
	// inject our test config after defaults are loaded but before Init.
	stdApp := app.(*modular.StdApplication)
	stdApp.OnConfigLoaded(func(_ modular.Application) error {
		app.RegisterConfigSection("httpclient", modular.NewStdConfigProvider(&httpclient.Config{
			Verbose:            true,
			DisableCompression: true,
			VerboseOptions: &httpclient.VerboseOptions{
				LogHeaders: true,
				LogBody:    true,
			},
		}))
		return nil
	})

	app.RegisterModule(httpclient.NewHTTPClientModule())

	// 4. Init wires the httpclient module (builds the loggingTransport).
	err = app.Init()
	require.NoError(t, err)

	// 5. Retrieve the *http.Client from the service registry — this is the
	// same client that reverseproxy receives via Constructor and uses as
	// proxy.Transport = m.httpClient.Transport (module.go:1581).
	var client *http.Client
	err = stdApp.GetService("httpclient", &client)
	require.NoError(t, err)
	require.NotNil(t, client, "httpclient service should provide *http.Client")
	require.NotNil(t, client.Transport, "httpclient should configure a custom transport")

	// Verify the transport is NOT a plain *http.Transport (should be loggingTransport).
	_, isPlainTransport := client.Transport.(*http.Transport)
	require.False(t, isPlainTransport,
		"expected httpclient to wrap the transport with loggingTransport when Verbose is true")

	// 6. Build a reverse proxy that uses the httpclient's transport, exactly
	// as the reverseproxy module does in createReverseProxyForBackend.
	backendURL, err := url.Parse(backend.URL)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.Transport = client.Transport

	// 7. Serve a request through the proxy.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	proxy.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// 8. Verify the response body is the raw gzip bytes (DisableCompression
	// ensures the transport does not auto-decompress).
	respBody, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Equal(t, gzBody, respBody, "proxy must pass gzip bytes through unchanged")

	// 9. Assert the log buffer contains the body-omission notice.
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "[body omitted: Content-Encoding=gzip",
		"expected gzip body-omission notice in logs")

	// 10. Assert the log buffer does NOT contain gzip magic bytes (\x1f\x8b).
	assert.False(t, strings.Contains(logOutput, "\x1f\x8b"),
		"log output must not contain raw gzip magic bytes")
}
