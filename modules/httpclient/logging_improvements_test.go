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
				assert.Contains(t, details, "Authorization", "Should show authorization header")

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
