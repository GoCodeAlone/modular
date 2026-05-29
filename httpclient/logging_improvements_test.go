package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogger captures log messages for testing
type TestLogger struct {
	entries []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	KeyVals map[string]interface{}
}

func (l *TestLogger) Debug(msg string, keyvals ...interface{}) {
	l.addEntry("DEBUG", msg, keyvals...)
}

func (l *TestLogger) Info(msg string, keyvals ...interface{}) {
	l.addEntry("INFO", msg, keyvals...)
}

func (l *TestLogger) Warn(msg string, keyvals ...interface{}) {
	l.addEntry("WARN", msg, keyvals...)
}

func (l *TestLogger) Error(msg string, keyvals ...interface{}) {
	l.addEntry("ERROR", msg, keyvals...)
}

func (l *TestLogger) addEntry(level, msg string, keyvals ...interface{}) {
	kvMap := make(map[string]interface{})
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			kvMap[fmt.Sprintf("%v", keyvals[i])] = keyvals[i+1]
		}
	}
	l.entries = append(l.entries, LogEntry{
		Level:   level,
		Message: msg,
		KeyVals: kvMap,
	})
}

func (l *TestLogger) GetEntries() []LogEntry {
	return l.entries
}

func (l *TestLogger) Clear() {
	l.entries = nil
}

// TestLoggingImprovements tests the improved logging functionality
func TestLoggingImprovements(t *testing.T) {
	tests := []struct {
		name             string
		logHeaders       bool
		logBody          bool
		maxBodyLogSize   int
		expectedBehavior string
	}{
		{
			name:             "Headers and body disabled - should show useful basic info",
			logHeaders:       false,
			logBody:          false,
			maxBodyLogSize:   0,
			expectedBehavior: "basic_info_with_important_headers",
		},
		{
			name:             "Headers and body enabled with zero size - should show smart truncation",
			logHeaders:       true,
			logBody:          true,
			maxBodyLogSize:   0,
			expectedBehavior: "smart_truncation_with_useful_info",
		},
		{
			name:             "Headers and body enabled with small size - should show truncated content",
			logHeaders:       true,
			logBody:          true,
			maxBodyLogSize:   20,
			expectedBehavior: "truncated_with_content",
		},
		{
			name:             "Headers and body enabled with large size - should show full content",
			logHeaders:       true,
			logBody:          true,
			maxBodyLogSize:   1000,
			expectedBehavior: "full_content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Custom-Header", "test-value")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"message": "Hello, World!"}`))
			}))
			defer server.Close()

			// Setup test logger
			testLogger := &TestLogger{}

			// Create logging transport
			transport := &loggingTransport{
				Transport:      http.DefaultTransport,
				Logger:         testLogger,
				FileLogger:     nil, // No file logging for these tests
				LogHeaders:     tt.logHeaders,
				LogBody:        tt.logBody,
				MaxBodyLogSize: tt.maxBodyLogSize,
				LogToFile:      false,
			}

			// Create client and make request
			client := &http.Client{Transport: transport}

			reqBody := bytes.NewBufferString(`{"test": "data"}`)
			req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL+"/api/test", reqBody)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer token123")
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify logging behavior
			entries := testLogger.GetEntries()

			// Should have at least request and response entries
			require.GreaterOrEqual(t, len(entries), 2, "Should have at least request and response log entries")

			// Find request and response entries
			var requestEntry, responseEntry *LogEntry
			for i := range entries {
				if strings.Contains(entries[i].Message, "Outgoing request") {
					requestEntry = &entries[i]
				}
				if strings.Contains(entries[i].Message, "Received response") {
					responseEntry = &entries[i]
				}
			}

			require.NotNil(t, requestEntry, "Should have request log entry")
			require.NotNil(t, responseEntry, "Should have response log entry")

			// Verify expected behavior
			switch tt.expectedBehavior {
			case "basic_info_with_important_headers":
				// Should show basic info and important headers even without detailed logging
				assert.Contains(t, fmt.Sprintf("%v", requestEntry.KeyVals["request"]), "POST")
				assert.Contains(t, fmt.Sprintf("%v", requestEntry.KeyVals["request"]), server.URL)
				assert.NotNil(t, requestEntry.KeyVals["important_headers"], "Should include important headers")

				assert.Contains(t, fmt.Sprintf("%v", responseEntry.KeyVals["response"]), "200")
				assert.NotNil(t, responseEntry.KeyVals["duration_ms"], "Should include timing")
				assert.NotNil(t, responseEntry.KeyVals["important_headers"], "Should include important response headers")

			case "smart_truncation_with_useful_info":
				// Should show full content because MaxBodyLogSize=0 triggers smart behavior
				assert.NotNil(t, requestEntry.KeyVals["details"], "Should include request details")
				assert.NotNil(t, responseEntry.KeyVals["details"], "Should include response details")

				details := fmt.Sprintf("%v", requestEntry.KeyVals["details"])
				assert.Contains(t, details, "POST", "Should show method")
				// The Authorization header NAME should still be visible for observability,
				// but its secret VALUE must be redacted (go/clear-text-logging).
				assert.Contains(t, details, "Authorization", "Should show authorization header name")
				assert.NotContains(t, details, "token123", "Authorization secret value must be redacted")

			case "truncated_with_content":
				// Should show truncated content with [truncated] marker
				assert.NotNil(t, requestEntry.KeyVals["details"], "Should include request details")
				assert.NotNil(t, responseEntry.KeyVals["details"], "Should include response details")

				reqDetails := fmt.Sprintf("%v", requestEntry.KeyVals["details"])
				respDetails := fmt.Sprintf("%v", responseEntry.KeyVals["details"])
				assert.Contains(t, reqDetails, "[truncated]", "Request should be marked as truncated")
				assert.Contains(t, respDetails, "[truncated]", "Response should be marked as truncated")

				// Should still contain useful information, not just "..."
				assert.Contains(t, reqDetails, "POST", "Truncated request should still show method")
				assert.Contains(t, respDetails, "HTTP", "Truncated response should still show status line")

			case "full_content":
				// Should show complete request and response
				assert.NotNil(t, requestEntry.KeyVals["details"], "Should include request details")
				assert.NotNil(t, responseEntry.KeyVals["details"], "Should include response details")

				reqDetails := fmt.Sprintf("%v", requestEntry.KeyVals["details"])
				respDetails := fmt.Sprintf("%v", responseEntry.KeyVals["details"])
				assert.NotContains(t, reqDetails, "[truncated]", "Request should not be truncated")
				assert.NotContains(t, respDetails, "[truncated]", "Response should not be truncated")

				// Should contain full HTTP content
				assert.Contains(t, reqDetails, "POST /api/test HTTP/1.1", "Should show full request line")
				assert.Contains(t, reqDetails, `{"test": "data"}`, "Should show request body")
				assert.True(t,
					strings.Contains(respDetails, "HTTP/1.1 200 OK") || strings.Contains(respDetails, "HTTP 200 OK"),
					"Should show status line, got: %s", respDetails)
				assert.Contains(t, respDetails, `{"message": "Hello, World!"}`, "Should show response body")
			}

			// Verify that timing is included in response
			assert.NotNil(t, responseEntry.KeyVals["duration_ms"], "Response should include timing information")

			// Verify that we're not generating too many log entries (original issue: minimize log entries)
			assert.LessOrEqual(t, len(entries), 3, "Should not generate excessive log entries")
		})
	}
}

// TestNoUselessDotDotDotLogs tests that we don't generate logs with just "..."
func TestNoUselessDotDotDotLogs(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Setup test logger
	testLogger := &TestLogger{}

	// Create logging transport with zero max body size (the original problem scenario)
	transport := &loggingTransport{
		Transport:      http.DefaultTransport,
		Logger:         testLogger,
		FileLogger:     nil,
		LogHeaders:     true,
		LogBody:        true,
		MaxBodyLogSize: 0, // This was the problem: caused logs with just "..."
		LogToFile:      false,
	}

	// Make a request
	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check all log entries
	entries := testLogger.GetEntries()

	for _, entry := range entries {
		// Check all key-value pairs for useless "..." content
		for key, value := range entry.KeyVals {
			valueStr := fmt.Sprintf("%v", value)

			// The original issue: logs that just contain "..." with no useful information
			if valueStr == "..." {
				t.Errorf("Found useless log entry with just '...' in key '%s': %+v", key, entry)
			}

			// Also check for the specific problematic patterns from the original issue
			if strings.Contains(entry.Message, "Request dump") && valueStr == "..." {
				t.Errorf("Found the original problematic 'Request dump' log with just '...': %+v", entry)
			}
			if strings.Contains(entry.Message, "Response dump") && valueStr == "..." {
				t.Errorf("Found the original problematic 'Response dump' log with just '...': %+v", entry)
			}
		}

		// Verify that truncated logs still contain useful information
		for key, value := range entry.KeyVals {
			valueStr := fmt.Sprintf("%v", value)
			if strings.Contains(valueStr, "[truncated]") {
				// If something is truncated, it should still contain useful information before the [truncated] marker
				truncatedContent := strings.Split(valueStr, " [truncated]")[0]
				assert.NotEmpty(t, strings.TrimSpace(truncatedContent),
					"Truncated content should not be empty, key: %s, entry: %+v", key, entry)

				// For HTTP requests/responses, truncated content should contain meaningful info
				if key == "details" {
					assert.True(t,
						strings.Contains(truncatedContent, "GET") ||
							strings.Contains(truncatedContent, "POST") ||
							strings.Contains(truncatedContent, "HTTP") ||
							strings.Contains(truncatedContent, "200") ||
							strings.Contains(truncatedContent, "404"),
						"Truncated HTTP content should contain method, protocol, or status code, got: %s", truncatedContent)
				}
			}
		}
	}

	// Ensure we actually have some log entries to test
	assert.GreaterOrEqual(t, len(entries), 2, "Should have generated some log entries to test")
}

// TestSensitiveHeadersRedacted verifies that Authorization and other sensitive headers are
// redacted in both request and response important_headers logging (go/clear-text-logging fix).
func TestSensitiveHeadersRedacted(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=abc123; HttpOnly")
		w.Header().Set("Authorization", "Bearer server-token") // unusual but tests redaction
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	testLogger := &TestLogger{}

	// Use non-detailed logging path (LogHeaders=false, LogBody=false) so the
	// important_headers map is populated and can be inspected.
	transport := &loggingTransport{
		Transport:  http.DefaultTransport,
		Logger:     testLogger,
		LogHeaders: false,
		LogBody:    false,
		LogToFile:  false,
	}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret-token-value")
	req.Header.Set("Cookie", "session=hunter2")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	entries := testLogger.GetEntries()
	require.GreaterOrEqual(t, len(entries), 2)

	// Find request entry
	var reqEntry, respEntry *LogEntry
	for i := range entries {
		if strings.Contains(entries[i].Message, "Outgoing request") {
			reqEntry = &entries[i]
		}
		if strings.Contains(entries[i].Message, "Received response") {
			respEntry = &entries[i]
		}
	}

	// Check request headers: Authorization and Cookie must be redacted
	require.NotNil(t, reqEntry, "must have request log entry")
	reqHeaders, ok := reqEntry.KeyVals["important_headers"]
	require.True(t, ok, "request entry must have important_headers key")
	reqHeadersStr := fmt.Sprintf("%v", reqHeaders)
	assert.NotContains(t, reqHeadersStr, "secret-token-value", "Authorization value must not appear in logs")
	assert.NotContains(t, reqHeadersStr, "hunter2", "Cookie value must not appear in logs")
	// The key name may appear (that's fine), but the value must be masked
	if strings.Contains(reqHeadersStr, "Authorization") || strings.Contains(reqHeadersStr, "authorization") {
		assert.Contains(t, reqHeadersStr, "***", "masked sentinel must be present")
	}

	// Check response headers: Set-Cookie must be redacted
	require.NotNil(t, respEntry, "must have response log entry")
	respHeaders, ok := respEntry.KeyVals["important_headers"]
	require.True(t, ok, "response entry must have important_headers key")
	respHeadersStr := fmt.Sprintf("%v", respHeaders)
	assert.NotContains(t, respHeadersStr, "abc123", "Set-Cookie value must not appear in logs")
}

// TestSensitiveHeadersRedactedInDetailedDump verifies that when detailed logging is
// enabled (LogHeaders=true), the raw HTTP dump emitted as "details" has sensitive
// header VALUES redacted while header names and the body remain intact
// (go/clear-text-logging fix for the detailed-logging path).
func TestSensitiveHeadersRedactedInDetailedDump(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=resp-secret-cookie; HttpOnly")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Hello, World!"}`))
	}))
	defer server.Close()

	testLogger := &TestLogger{}

	// Detailed logging enabled: dumps the full request/response including headers.
	transport := &loggingTransport{
		Transport:      http.DefaultTransport,
		Logger:         testLogger,
		LogHeaders:     true,
		LogBody:        true,
		MaxBodyLogSize: 4096, // large enough to avoid truncation
		LogToFile:      false,
	}
	client := &http.Client{Transport: transport}

	reqBody := bytes.NewBufferString(`{"test": "data"}`)
	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL+"/api/test", reqBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer secret-token-value")
	req.Header.Set("Cookie", "session=req-secret-cookie")
	req.Header.Set("X-Api-Key", "my-api-key-secret")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	entries := testLogger.GetEntries()
	var reqEntry, respEntry *LogEntry
	for i := range entries {
		if strings.Contains(entries[i].Message, "Outgoing request") {
			reqEntry = &entries[i]
		}
		if strings.Contains(entries[i].Message, "Received response") {
			respEntry = &entries[i]
		}
	}

	require.NotNil(t, reqEntry, "must have request log entry")
	reqDetails := fmt.Sprintf("%v", reqEntry.KeyVals["details"])

	// Secret VALUES must never appear in the dump.
	assert.NotContains(t, reqDetails, "secret-token-value", "Authorization bearer token must be redacted in dump")
	assert.NotContains(t, reqDetails, "req-secret-cookie", "Cookie value must be redacted in dump")
	assert.NotContains(t, reqDetails, "my-api-key-secret", "X-Api-Key value must be redacted in dump")

	// Header NAMES and the body should still be present (observability preserved).
	assert.Contains(t, reqDetails, "Authorization", "Authorization header name should remain")
	assert.Contains(t, reqDetails, "***", "redaction sentinel must be present")
	assert.Contains(t, reqDetails, `{"test": "data"}`, "request body must be preserved")
	assert.Contains(t, reqDetails, "POST /api/test HTTP/1.1", "request line must be preserved")

	require.NotNil(t, respEntry, "must have response log entry")
	respDetails := fmt.Sprintf("%v", respEntry.KeyVals["details"])
	assert.NotContains(t, respDetails, "resp-secret-cookie", "Set-Cookie value must be redacted in dump")
	assert.Contains(t, respDetails, "Set-Cookie", "Set-Cookie header name should remain")
	assert.Contains(t, respDetails, `{"message": "Hello, World!"}`, "response body must be preserved")
}
