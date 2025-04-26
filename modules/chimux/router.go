package chimux

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

// RouterService defines the interface for working with the Chi router
type RouterService interface {
	// Standard HTTP method handlers
	Get(pattern string, handler http.HandlerFunc)
	Post(pattern string, handler http.HandlerFunc)
	Put(pattern string, handler http.HandlerFunc)
	Delete(pattern string, handler http.HandlerFunc)
	Patch(pattern string, handler http.HandlerFunc)
	Head(pattern string, handler http.HandlerFunc)
	Options(pattern string, handler http.HandlerFunc)

	// Route creates a new sub-router for the given pattern
	Route(pattern string, fn func(r Router))

	// Mount attaches another http.Handler at the given pattern
	Mount(pattern string, handler http.Handler)

	// Use appends middleware to the chain
	Use(middleware ...func(http.Handler) http.Handler)

	// Handle registers a handler for a specific pattern
	Handle(pattern string, handler http.Handler)

	// HandleFunc registers a handler function for a specific pattern
	HandleFunc(pattern string, handler http.HandlerFunc)
}

// Router is the sub-router interface
type Router interface {
	RouterService
}

// ChiRouterService extends RouterService with methods to access the underlying chi router
type ChiRouterService interface {
	RouterService
	ChiRouter() chi.Router
}

// Middleware is an alias for the chi middleware handler function
type Middleware func(http.Handler) http.Handler

// MiddlewareProvider defines a service that provides middleware for the chimux router
type MiddlewareProvider interface {
	ProvideMiddleware() []Middleware
}
