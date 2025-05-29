package httpclient

import (
	"net/http"
)

// ClientService defines the interface for the HTTP client service.
// Any module that needs to make HTTP requests can use this service.
type ClientService interface {
	// Client returns the configured http.Client instance
	Client() *http.Client

	// RequestModifier returns a modifier function that can modify a request before it's sent
	RequestModifier() RequestModifierFunc

	// WithTimeout creates a new client with the specified timeout in seconds
	WithTimeout(timeoutSeconds int) *http.Client
}

// RequestModifierFunc is a function type that can be used to modify an HTTP request
// before it is sent by the client.
type RequestModifierFunc func(*http.Request) *http.Request
