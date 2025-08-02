// Package httpclient provides a configurable HTTP client module for the modular framework.
//
// This module offers a production-ready HTTP client with comprehensive configuration
// options, request/response logging, connection pooling, timeout management, and
// request modification capabilities. It's designed for reliable HTTP communication
// in microservices and web applications.
//
// # Features
//
// The httpclient module provides the following capabilities:
//   - Configurable connection pooling and keep-alive settings
//   - Request and response timeout management
//   - TLS handshake timeout configuration
//   - Comprehensive request/response logging with file output
//   - Request modification pipeline for adding headers, authentication, etc.
//   - Performance-optimized transport settings
//   - Support for compression and keep-alive control
//   - Service interface for dependency injection
//
// # Configuration
//
// The module can be configured through the Config structure:
//
//	config := &Config{
//	    MaxIdleConns:        100,        // total idle connections
//	    MaxIdleConnsPerHost: 10,         // idle connections per host
//	    IdleConnTimeout:     90,         // idle connection timeout (seconds)
//	    RequestTimeout:      30,         // request timeout (seconds)
//	    TLSTimeout:          10,         // TLS handshake timeout (seconds)
//	    DisableCompression:  false,      // enable gzip compression
//	    DisableKeepAlives:   false,      // enable connection reuse
//	    Verbose:             true,       // enable request/response logging
//	    VerboseOptions: &VerboseOptions{
//	        LogToFile:    true,
//	        LogFilePath:  "/var/log/httpclient",
//	    },
//	}
//
// # Service Registration
//
// The module registers itself as a service for dependency injection:
//
//	// Get the HTTP client service
//	client := app.GetService("httpclient").(httpclient.ClientService)
//
//	// Use the client
//	resp, err := client.Client().Get("https://api.example.com/users")
//
//	// Create a client with custom timeout
//	timeoutClient := client.WithTimeout(60)
//	resp, err := timeoutClient.Post("https://api.example.com/upload", "application/json", data)
//
// # Usage Examples
//
// Basic HTTP requests:
//
//	// GET request
//	resp, err := client.Client().Get("https://api.example.com/health")
//	if err != nil {
//	    return err
//	}
//	defer resp.Body.Close()
//
//	// POST request with JSON
//	jsonData := bytes.NewBuffer([]byte(`{"name": "test"}`))
//	resp, err := client.Client().Post(
//	    "https://api.example.com/users",
//	    "application/json",
//	    jsonData,
//	)
//
// Request modification for authentication:
//
//	// Set up request modifier for API key authentication
//	modifier := func(req *http.Request) *http.Request {
//	    req.Header.Set("Authorization", "Bearer "+apiToken)
//	    req.Header.Set("User-Agent", "MyApp/1.0")
//	    return req
//	}
//	client.SetRequestModifier(modifier)
//
//	// All subsequent requests will include the headers
//	resp, err := client.Client().Get("https://api.example.com/protected")
//
// Custom timeout scenarios:
//
//	// Short timeout for health checks
//	healthClient := client.WithTimeout(5)
//	resp, err := healthClient.Get("https://service.example.com/health")
//
//	// Long timeout for file uploads
//	uploadClient := client.WithTimeout(300)
//	resp, err := uploadClient.Post("https://api.example.com/upload", contentType, fileData)
//
// # Logging and Debugging
//
// When verbose logging is enabled, the module logs detailed request and response
// information including headers, bodies, and timing data. This is invaluable for
// debugging API integrations and monitoring HTTP performance.
//
// Log output includes:
//   - Request method, URL, and headers
//   - Request body (configurable)
//   - Response status, headers, and body
//   - Request duration and timing breakdown
//   - Error details and retry information
//
// # Performance Considerations
//
// The module is optimized for production use with:
//   - Connection pooling to reduce connection overhead
//   - Keep-alive connections for better performance
//   - Configurable timeouts to prevent resource leaks
//   - Optional compression to reduce bandwidth usage
//   - Efficient request modification pipeline
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ModuleName is the unique identifier for the httpclient module.
const ModuleName = "httpclient"

// ServiceName is the name of the service provided by this module.
// Other modules can use this name to request the HTTP client service through dependency injection.
const ServiceName = "httpclient"

// HTTPClientModule implements a configurable HTTP client module.
// It provides a production-ready HTTP client with comprehensive configuration
// options, logging capabilities, and request modification features.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - ClientService: HTTP client service interface
//
// The HTTP client is thread-safe and can be used concurrently from multiple goroutines.
type HTTPClientModule struct {
	config     *Config
	app        modular.Application
	logger     modular.Logger
	fileLogger *FileLogger
	httpClient *http.Client
	transport  *http.Transport
	modifier   RequestModifierFunc
}

// Make sure HTTPClientModule implements necessary interfaces
var (
	_ modular.Module = (*HTTPClientModule)(nil)
	_ ClientService  = (*HTTPClientModule)(nil)
)

// NewHTTPClientModule creates a new instance of the HTTP client module.
// This is the primary constructor for the httpclient module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(httpclient.NewHTTPClientModule())
func NewHTTPClientModule() modular.Module {
	return &HTTPClientModule{
		modifier: func(r *http.Request) *http.Request { return r }, // Default no-op modifier
	}
}

// Name returns the unique identifier for this module.
// This name is used for service registration, dependency resolution,
// and configuration section identification.
func (m *HTTPClientModule) Name() string {
	return ModuleName
}

// RegisterConfig registers the module's configuration structure.
// This method is called during application initialization to register
// the default configuration values for the httpclient module.
//
// Default configuration:
//   - MaxIdleConns: 100 (total idle connections)
//   - MaxIdleConnsPerHost: 10 (idle connections per host)
//   - IdleConnTimeout: 90 seconds
//   - RequestTimeout: 30 seconds
//   - TLSTimeout: 10 seconds
//   - DisableCompression: false (compression enabled)
//   - DisableKeepAlives: false (keep-alives enabled)
//   - Verbose: false (logging disabled)
func (m *HTTPClientModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90,
		RequestTimeout:      30,
		TLSTimeout:          10,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		Verbose:             false,
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the httpclient module with the application context.
// This method is called after all modules have been registered and their
// configurations loaded. It sets up the HTTP client, transport, and logging.
//
// The initialization process:
//  1. Retrieves the module's configuration
//  2. Sets up logging
//  3. Creates and configures the HTTP transport with connection pooling
//  4. Sets up request/response logging if verbose mode is enabled
//  5. Creates the HTTP client with configured transport and middleware
//  6. Initializes request modification pipeline
//
// Transport configuration includes:
//   - Connection pooling settings for optimal performance
//   - Timeout configurations for reliability
//   - Compression and keep-alive settings
//   - TLS handshake timeout for secure connections
func (m *HTTPClientModule) Init(app modular.Application) error {
	m.app = app
	m.logger = app.Logger()
	m.logger.Info("Initializing HTTP client module")

	// Get the config section
	cfg, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.Name(), err)
	}
	m.config = cfg.GetConfig().(*Config)

	// Create the transport with the configured settings
	m.transport = &http.Transport{
		MaxIdleConns:        m.config.MaxIdleConns,
		MaxIdleConnsPerHost: m.config.MaxIdleConnsPerHost,
		IdleConnTimeout:     m.config.GetTimeout(m.config.IdleConnTimeout),
		TLSHandshakeTimeout: m.config.GetTimeout(m.config.TLSTimeout),
		DisableCompression:  m.config.DisableCompression,
		DisableKeepAlives:   m.config.DisableKeepAlives,
	}

	// Create the HTTP client with the transport
	baseTransport := http.RoundTripper(m.transport)

	// If verbose logging is enabled, wrap the transport with logging
	if m.config.Verbose {
		// If we should log to file, initialize the file logger
		if m.config.VerboseOptions.LogToFile {
			var logFilePath string
			if m.config.VerboseOptions.LogFilePath == "" {
				logFilePath = "httpclient_logs" // Default directory
				m.logger.Warn("Log file path not specified, using default",
					"path", logFilePath,
				)
			} else {
				logFilePath = m.config.VerboseOptions.LogFilePath
			}

			// Create the file logger
			fileLogger, err := NewFileLogger(logFilePath, m.logger)
			if err != nil {
				m.logger.Error("Failed to create file logger",
					"path", logFilePath,
					"error", err,
				)
			} else {
				m.fileLogger = fileLogger
				m.logger.Info("HTTP client file logging enabled",
					"path", logFilePath,
				)
			}
		}

		baseTransport = &loggingTransport{
			Transport:      baseTransport,
			Logger:         m.logger,
			FileLogger:     m.fileLogger,
			LogHeaders:     m.config.VerboseOptions.LogHeaders,
			LogBody:        m.config.VerboseOptions.LogBody,
			MaxBodyLogSize: m.config.VerboseOptions.MaxBodyLogSize,
			LogToFile:      m.config.VerboseOptions.LogToFile && m.fileLogger != nil,
		}
	}

	m.httpClient = &http.Client{
		Transport: baseTransport,
		Timeout:   m.config.GetTimeout(m.config.RequestTimeout),
	}

	return nil
}

// Start performs startup logic for the module.
func (m *HTTPClientModule) Start(context.Context) error {
	m.logger.Info("Starting HTTP client module")
	return nil
}

// Stop performs shutdown logic for the module.
func (m *HTTPClientModule) Stop(context.Context) error {
	m.logger.Info("Stopping HTTP client module")
	m.transport.CloseIdleConnections()

	// Close the file logger if it exists
	if m.fileLogger != nil {
		if closeErr := m.fileLogger.Close(); closeErr != nil {
			m.logger.Warn("Failed to close file logger", "error", closeErr)
		}
	}

	return nil
}

// ProvidesServices returns services provided by this module.
func (m *HTTPClientModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "HTTP client (*http.Client) for direct usage",
			Instance:    m.httpClient, // Provide the actual *http.Client instance
		},
		{
			Name:        "httpclient-service",
			Description: "HTTP client service interface (ClientService) for advanced features",
			Instance:    m, // Provide the service interface for modules that need additional features
		},
	}
}

// RequiresServices returns services required by this module.
func (m *HTTPClientModule) RequiresServices() []modular.ServiceDependency {
	return nil // No service dependencies
}

// Dependencies returns the names of modules this module depends on.
func (m *HTTPClientModule) Dependencies() []string {
	return nil // No module dependencies
}

// Client returns the configured http.Client instance.
func (m *HTTPClientModule) Client() *http.Client {
	return m.httpClient
}

// RequestModifier returns a modifier function that can modify a request before it's sent.
func (m *HTTPClientModule) RequestModifier() RequestModifierFunc {
	return m.modifier
}

// WithTimeout creates a new client with the specified timeout in seconds.
func (m *HTTPClientModule) WithTimeout(timeoutSeconds int) *http.Client {
	if timeoutSeconds <= 0 {
		return m.httpClient // Return the default client
	}

	// Create a new client with the specified timeout
	return &http.Client{
		Transport: m.httpClient.Transport,
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
	}
}

// SetRequestModifier sets the request modifier function.
func (m *HTTPClientModule) SetRequestModifier(modifier RequestModifierFunc) {
	if modifier != nil {
		m.modifier = modifier
	}
}

// loggingTransport provides verbose logging of HTTP requests and responses.
type loggingTransport struct {
	Transport      http.RoundTripper
	Logger         modular.Logger
	FileLogger     *FileLogger
	LogHeaders     bool
	LogBody        bool
	MaxBodyLogSize int
	LogToFile      bool
}

// RoundTrip implements the http.RoundTripper interface and adds logging.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Track request ID and timing
	requestID := fmt.Sprintf("%p", req)
	startTime := time.Now()

	// Log the request
	t.logRequest(requestID, req)

	// Execute the actual request
	resp, err := t.Transport.RoundTrip(req)

	// Calculate timing
	duration := time.Since(startTime)

	// Log error if any occurred
	if err != nil {
		t.Logger.Error("Request failed",
			"id", requestID,
			"url", req.URL.String(),
			"method", req.Method,
			"duration_ms", duration.Milliseconds(),
			"error", err,
		)
		return resp, fmt.Errorf("http request failed: %w", err)
	}

	// Log the response (timing will be included in response log)
	t.logResponse(requestID, req.URL.String(), resp, duration)

	// Handle file logging if enabled
	if t.LogToFile && t.FileLogger != nil {
		t.handleFileLogging(requestID, req, resp, duration)
	}

	return resp, nil
}

// logRequest logs detailed information about the request.
func (t *loggingTransport) logRequest(id string, req *http.Request) {
	// Basic request information that's always useful
	basicInfo := fmt.Sprintf("%s %s", req.Method, req.URL.String())

	// If detailed logging is enabled, try to get more information
	if t.LogHeaders || t.LogBody {
		dumpBody := t.LogBody
		reqDump, err := httputil.DumpRequestOut(req, dumpBody)
		if err != nil {
			// If dump fails, log basic info with error
			t.Logger.Info("Outgoing request (dump failed)",
				"id", id,
				"request", basicInfo,
				"error", err,
			)
		} else {
			if t.LogToFile && t.FileLogger != nil {
				// Log to file using our FileLogger
				t.Logger.Info("Outgoing request (logged to file)",
					"id", id,
					"request", basicInfo,
				)
				if err := t.FileLogger.LogRequest(id, reqDump); err != nil {
					t.Logger.Error("Failed to write request to log file",
						"id", id,
						"error", err,
					)
				}
			} else {
				// Log to application logger with smart truncation
				dumpStr := string(reqDump)
				if t.MaxBodyLogSize > 0 && len(reqDump) > t.MaxBodyLogSize {
					// Smart truncation: try to include the request line and headers
					truncated := t.smartTruncateRequest(dumpStr, t.MaxBodyLogSize)
					t.Logger.Info("Outgoing request",
						"id", id,
						"request", basicInfo,
						"details", truncated+" [truncated]",
					)
				} else {
					t.Logger.Info("Outgoing request",
						"id", id,
						"request", basicInfo,
						"details", dumpStr,
					)
				}
			}
		}
	} else {
		// Even when detailed logging is disabled, show useful basic information
		headers := make(map[string]string)
		for key, values := range req.Header {
			if len(values) > 0 && t.isImportantHeader(key) {
				headers[key] = values[0]
			}
		}

		t.Logger.Info("Outgoing request",
			"id", id,
			"request", basicInfo,
			"content_length", req.ContentLength,
			"important_headers", headers,
		)
	}
}

// logResponse logs detailed information about the response.
func (t *loggingTransport) logResponse(id, url string, resp *http.Response, duration time.Duration) {
	if resp == nil {
		t.Logger.Warn("Nil response received",
			"id", id,
			"url", url,
			"duration_ms", duration.Milliseconds(),
		)
		return
	}

	// Basic response information that's always useful
	basicInfo := fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	// If detailed logging is enabled, try to get more information
	if t.LogHeaders || t.LogBody {
		// If we need to log the body, we must read it and restore it for the caller
		var respDump []byte
		var err error
		var bodyBytes []byte

		if t.LogBody && resp.Body != nil {
			// Read body for logging
			bodyBytes, err = io.ReadAll(resp.Body)
			if err != nil {
				t.Logger.Info("Received response (body read failed)",
					"id", id,
					"response", basicInfo,
					"url", url,
					"duration_ms", duration.Milliseconds(),
					"error", err,
				)
				return
			}

			// Restore the body for the caller
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Create the response dump manually
			respDump = append([]byte(fmt.Sprintf("HTTP %s\r\n", resp.Status)), []byte{}...)
			for k, v := range resp.Header {
				respDump = append(respDump, []byte(fmt.Sprintf("%s: %s\r\n", k, v[0]))...)
			}
			respDump = append(respDump, []byte("\r\n")...)
			respDump = append(respDump, bodyBytes...)

		} else {
			// If we don't need to log the body or there is no body,
			// we can use httputil.DumpResponse
			dumpBody := t.LogBody
			respDump, err = httputil.DumpResponse(resp, dumpBody)
		}

		if err != nil {
			// If dump fails, log basic info with error
			t.Logger.Info("Received response (dump failed)",
				"id", id,
				"response", basicInfo,
				"url", url,
				"duration_ms", duration.Milliseconds(),
				"error", err,
			)
		} else {
			if t.LogToFile && t.FileLogger != nil {
				// Log the response to file using our FileLogger
				t.Logger.Info("Received response (logged to file)",
					"id", id,
					"response", basicInfo,
					"url", url,
					"duration_ms", duration.Milliseconds(),
				)
				if err := t.FileLogger.LogResponse(id, respDump); err != nil {
					t.Logger.Error("Failed to write response to log file",
						"id", id,
						"error", err,
					)
				}
			} else {
				// Log to application logger with smart truncation
				dumpStr := string(respDump)
				if t.MaxBodyLogSize > 0 && len(respDump) > t.MaxBodyLogSize {
					// Smart truncation: try to include the status line and headers
					truncated := t.smartTruncateResponse(dumpStr, t.MaxBodyLogSize)
					t.Logger.Info("Received response",
						"id", id,
						"response", basicInfo,
						"url", url,
						"duration_ms", duration.Milliseconds(),
						"details", truncated+" [truncated]",
					)
				} else {
					t.Logger.Info("Received response",
						"id", id,
						"response", basicInfo,
						"url", url,
						"duration_ms", duration.Milliseconds(),
						"details", dumpStr,
					)
				}
			}
		}
	} else {
		// Even when detailed logging is disabled, show useful basic information
		headers := make(map[string]string)
		for key, values := range resp.Header {
			if len(values) > 0 && t.isImportantHeader(key) {
				headers[key] = values[0]
			}
		}

		t.Logger.Info("Received response",
			"id", id,
			"response", basicInfo,
			"url", url,
			"duration_ms", duration.Milliseconds(),
			"content_length", resp.ContentLength,
			"important_headers", headers,
		)
	}
}

// smartTruncateRequest intelligently truncates a request dump to fit within maxSize
// while preserving the most important information (request line and key headers).
func (t *loggingTransport) smartTruncateRequest(dump string, maxSize int) string {
	if maxSize <= 0 {
		// Extract just the request line and essential headers
		lines := strings.Split(dump, "\n")
		if len(lines) == 0 {
			return ""
		}

		result := lines[0] // Request line (e.g., "POST /api/test HTTP/1.1")

		// Add key headers if space permits
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				break // End of headers
			}
			if t.isImportantHeaderLine(line) && len(result)+len(line)+2 < 200 {
				result += "\n" + line
			}
		}

		return result
	}

	if len(dump) <= maxSize {
		return dump
	}

	// Try to include headers by finding the body separator
	headerEnd := strings.Index(dump, "\n\n")
	if headerEnd == -1 {
		headerEnd = strings.Index(dump, "\r\n\r\n")
		if headerEnd != -1 {
			headerEnd += 2
		}
	}

	if headerEnd > 0 && headerEnd <= maxSize {
		// Include headers and part of body
		remaining := maxSize - headerEnd - 2
		if remaining > 0 && len(dump) > headerEnd+2 {
			bodyStart := headerEnd + 2
			if bodyStart+remaining < len(dump) {
				return dump[:bodyStart+remaining]
			}
		}
		return dump[:headerEnd]
	}

	// Fallback: just truncate
	return dump[:maxSize]
}

// smartTruncateResponse intelligently truncates a response dump to fit within maxSize
// while preserving the most important information (status line and key headers).
func (t *loggingTransport) smartTruncateResponse(dump string, maxSize int) string {
	if maxSize <= 0 {
		// Extract just the status line and essential headers
		lines := strings.Split(dump, "\n")
		if len(lines) == 0 {
			return ""
		}

		result := lines[0] // Status line (e.g., "HTTP/1.1 200 OK")

		// Add key headers if space permits
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				break // End of headers
			}
			if t.isImportantHeaderLine(line) && len(result)+len(line)+2 < 200 {
				result += "\n" + line
			}
		}

		return result
	}

	if len(dump) <= maxSize {
		return dump
	}

	// Try to include headers by finding the body separator
	headerEnd := strings.Index(dump, "\n\n")
	if headerEnd == -1 {
		headerEnd = strings.Index(dump, "\r\n\r\n")
		if headerEnd != -1 {
			headerEnd += 2
		}
	}

	if headerEnd > 0 && headerEnd <= maxSize {
		// Include headers and part of body
		remaining := maxSize - headerEnd - 2
		if remaining > 0 && len(dump) > headerEnd+2 {
			bodyStart := headerEnd + 2
			if bodyStart+remaining < len(dump) {
				return dump[:bodyStart+remaining]
			}
		}
		return dump[:headerEnd]
	}

	// Fallback: just truncate
	return dump[:maxSize]
}

// isImportantHeader determines if a header is important enough to show
// even when detailed logging is disabled.
func (t *loggingTransport) isImportantHeader(headerName string) bool {
	important := []string{
		"content-type", "content-length", "user-agent",
		"accept", "cache-control", "x-request-id", "x-correlation-id",
		"x-trace-id", "location",
	}

	headerLower := strings.ToLower(headerName)
	for _, imp := range important {
		if headerLower == imp {
			return true
		}
	}
	return false
}

// isImportantHeaderLine determines if a header line is important based on its content.
func (t *loggingTransport) isImportantHeaderLine(line string) bool {
	colonIndex := strings.Index(line, ":")
	if colonIndex <= 0 {
		return false
	}
	headerName := line[:colonIndex]
	return t.isImportantHeader(headerName)
}

// handleFileLogging handles file-based logging for transactions.
func (t *loggingTransport) handleFileLogging(requestID string, req *http.Request, resp *http.Response, duration time.Duration) {
	if !t.LogHeaders && !t.LogBody {
		return // No detailed logging requested
	}

	// Get request dump
	dumpBody := t.LogBody
	reqDump, err := httputil.DumpRequestOut(req, dumpBody)
	if err != nil {
		t.Logger.Error("Failed to dump request for transaction logging",
			"id", requestID,
			"error", err,
		)
		return
	}

	// Get response dump
	var respDump []byte
	if t.LogBody && resp.Body != nil {
		// We need to read the body for logging and then restore it
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Logger.Error("Failed to read response body for transaction logging",
				"id", requestID,
				"error", err,
			)
			return
		}

		// Restore the body for the caller
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Create the response dump manually
		respDump = append([]byte(fmt.Sprintf("HTTP %s\r\n", resp.Status)), []byte{}...)
		for k, v := range resp.Header {
			respDump = append(respDump, []byte(fmt.Sprintf("%s: %s\r\n", k, v[0]))...)
		}
		respDump = append(respDump, []byte("\r\n")...)
		respDump = append(respDump, bodyBytes...)
	} else {
		// If we don't need the body or there is no body
		respDump, err = httputil.DumpResponse(resp, false)
		if err != nil {
			t.Logger.Error("Failed to dump response for transaction logging",
				"id", requestID,
				"error", err,
			)
			return
		}
	}

	// Write transaction log
	if err := t.FileLogger.LogTransactionToFile(requestID, reqDump, respDump, duration, req.URL.String()); err != nil {
		t.Logger.Error("Failed to write transaction to log file",
			"id", requestID,
			"error", err,
		)
	}
}
