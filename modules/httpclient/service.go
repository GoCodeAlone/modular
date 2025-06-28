package httpclient

import (
	"net/http"
)

// ClientService defines the interface for the HTTP client service.
// This interface provides access to configured HTTP clients and request
// modification capabilities. Any module that needs to make HTTP requests
// can use this service through dependency injection.
//
// The service provides multiple ways to access HTTP clients:
//   - Default client with module configuration
//   - Timeout-specific clients for different use cases
//   - Request modification pipeline for common headers/auth
//
// Example usage:
//
//	// Basic usage
//	client := httpClientService.Client()
//	resp, err := client.Get("https://api.example.com/data")
//
//	// Custom timeout
//	shortTimeoutClient := httpClientService.WithTimeout(5)
//	resp, err := shortTimeoutClient.Get("https://api.example.com/health")
//
//	// Request modification
//	modifier := httpClientService.RequestModifier()
//	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
//	modifiedReq := modifier(req)
type ClientService interface {
	// Client returns the configured http.Client instance.
	// This client uses the module's configuration for timeouts, connection
	// pooling, compression, and other transport settings. The client is
	// thread-safe and can be used concurrently.
	//
	// The returned client includes any configured request modification
	// pipeline and verbose logging if enabled.
	Client() *http.Client

	// RequestModifier returns a modifier function that can modify a request before it's sent.
	// This function applies any configured request modifications such as
	// authentication headers, user agents, or custom headers.
	//
	// The modifier can be used manually when creating custom requests:
	//	req, _ := http.NewRequest("POST", url, body)
	//	req = modifier(req)
	//	resp, err := client.Do(req)
	RequestModifier() RequestModifierFunc

	// WithTimeout creates a new client with the specified timeout in seconds.
	// This is useful for creating clients with different timeout requirements
	// without affecting the default client configuration.
	//
	// The new client inherits all other configuration from the module
	// (connection pooling, compression, etc.) but uses the specified timeout.
	//
	// Common timeout scenarios:
	//   - Health checks: 5-10 seconds
	//   - API calls: 30-60 seconds
	//   - File uploads: 300+ seconds
	WithTimeout(timeoutSeconds int) *http.Client
}

// RequestModifierFunc is a function type that can be used to modify an HTTP request
// before it is sent by the client.
//
// Request modifiers are useful for:
//   - Adding authentication headers (Bearer tokens, API keys)
//   - Setting common headers (User-Agent, Content-Type)
//   - Adding request tracking (correlation IDs, request IDs)
//   - Request logging and debugging
//   - Request validation and sanitization
//
// Example modifier implementations:
//
//	// API key authentication
//	func apiKeyModifier(apiKey string) RequestModifierFunc {
//	    return func(req *http.Request) *http.Request {
//	        req.Header.Set("Authorization", "Bearer "+apiKey)
//	        return req
//	    }
//	}
//
//	// Request tracing
//	func tracingModifier(req *http.Request) *http.Request {
//	    req.Header.Set("X-Request-ID", generateRequestID())
//	    req.Header.Set("X-Trace-ID", getTraceID(req.Context()))
//	    return req
//	}
//
//	// User agent setting
//	func userAgentModifier(userAgent string) RequestModifierFunc {
//	    return func(req *http.Request) *http.Request {
//	        req.Header.Set("User-Agent", userAgent)
//	        return req
//	    }
//	}
type RequestModifierFunc func(*http.Request) *http.Request
