package reverseproxy

import (
	"context"
	"net/http"
)

// CompositeResponse represents a transformed response from multiple backend requests
type CompositeResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// BackendEndpointRequest defines a request to be sent to a backend
type BackendEndpointRequest struct {
	Backend     string
	Method      string
	Path        string
	Headers     map[string]string
	QueryParams map[string]string
}

// EndpointMapping defines how requests should be routed to different backends
// and how their responses should be combined
type EndpointMapping struct {
	// Endpoints lists the backend requests to make
	Endpoints []BackendEndpointRequest

	// ResponseTransformer is a function that transforms multiple backend responses
	// into a single composite response
	ResponseTransformer func(ctx context.Context, req *http.Request, responses map[string]*http.Response) (*CompositeResponse, error)
}
