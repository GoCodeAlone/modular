package chimux

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ChiRouterService defines the interface for working with the Chi router
type ChiRouterService interface {
	// Direct access to the underlying chi router
	ChiRouter() chi.Router
}

// Middleware is an alias for the chi middleware handler function
type Middleware func(http.Handler) http.Handler

// MiddlewareProvider defines a service that provides middleware for the chimux router
type MiddlewareProvider interface {
	ProvideMiddleware() []Middleware
}

// BasicRouter defines the essential router interface that most modules need
// This interface avoids the Route/Group methods that are problematic for interface abstraction
type BasicRouter interface {
	// HTTP method handlers
	Get(pattern string, handler http.HandlerFunc)
	Post(pattern string, handler http.HandlerFunc)
	Put(pattern string, handler http.HandlerFunc)
	Delete(pattern string, handler http.HandlerFunc)
	Patch(pattern string, handler http.HandlerFunc)
	Head(pattern string, handler http.HandlerFunc)
	Options(pattern string, handler http.HandlerFunc)

	// Generic handlers
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler http.HandlerFunc)

	// Mounting and middleware
	Mount(pattern string, handler http.Handler)
	Use(middlewares ...func(http.Handler) http.Handler)

	// HTTP handler
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Router extends BasicRouter with Chi's actual interface
// This allows modules that need Route/Group to access them directly
type Router interface {
	BasicRouter
	chi.Router // Embed Chi's actual Router interface
}

// RouterService is an alias for BasicRouter for modules that don't need Route/Group
type RouterService = BasicRouter
