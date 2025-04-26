// Package reverseproxy provides a flexible reverse proxy module with support for multiple backends,
// composite responses, and tenant awareness.
package reverseproxy

import (
	"strings"
)

// PathMatcher helps determine which backend should handle a request based on path patterns.
// It maintains a mapping of backend IDs to URL path patterns and provides methods to
// register patterns and find matching backends for incoming requests.
type PathMatcher struct {
	// routePatterns maps backend IDs to their associated path patterns
	routePatterns map[string][]string // map of backendID to path patterns
}

// NewPathMatcher creates a new PathMatcher instance with initialized storage.
// This is used to create a fresh path matcher for route registration.
func NewPathMatcher() *PathMatcher {
	return &PathMatcher{
		routePatterns: make(map[string][]string),
	}
}

// AddRoutePattern adds a path pattern that should be routed to the specified backend.
// Multiple patterns can be registered for the same backend.
//
// Parameters:
//   - backendID: The identifier of the backend service to route to
//   - pattern: The URL path pattern to match (e.g., "/api/users")
func (pm *PathMatcher) AddRoutePattern(backendID, pattern string) {
	if _, ok := pm.routePatterns[backendID]; !ok {
		pm.routePatterns[backendID] = make([]string, 0)
	}
	pm.routePatterns[backendID] = append(pm.routePatterns[backendID], pattern)
}

// MatchBackend determines which backend should handle the given path.
// It checks if the path starts with any of the registered patterns and returns
// the matching backend ID. If multiple patterns match, the first match wins.
//
// Parameters:
//   - path: The request path to match against registered patterns
//
// Returns:
//   - The matching backendID or empty string if no match is found
func (pm *PathMatcher) MatchBackend(path string) string {
	for backendID, patterns := range pm.routePatterns {
		for _, pattern := range patterns {
			if strings.HasPrefix(path, pattern) {
				return backendID
			}
		}
	}
	return ""
}
