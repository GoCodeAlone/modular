package chimux

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ChiRouterService defines the interface for working with the Chi router.
// This interface provides direct access to the underlying Chi router instance
// for modules that need advanced Chi-specific functionality.
type ChiRouterService interface {
	// ChiRouter returns the underlying chi.Router instance.
	// Use this when you need access to Chi's advanced features like
	// Route, Group, or other Chi-specific methods.
	ChiRouter() chi.Router
}

// Middleware is an alias for the Chi middleware handler function.
// This type represents a middleware function that can be applied to routes.
type Middleware func(http.Handler) http.Handler

// MiddlewareProvider defines a service that provides middleware for the chimux router.
// Modules implementing this interface will have their middleware automatically
// discovered and applied to the router during initialization.
//
// Example implementation:
//
//	type AuthModule struct{}
//
//	func (a *AuthModule) ProvideMiddleware() []chimux.Middleware {
//	    return []chimux.Middleware{
//	        authenticationMiddleware,
//	        authorizationMiddleware,
//	    }
//	}
type MiddlewareProvider interface {
	// ProvideMiddleware returns a slice of middleware functions that should
	// be applied to the router. The middleware will be applied in the order
	// returned by this method.
	ProvideMiddleware() []Middleware
}

// BasicRouter defines the essential router interface that most modules need.
// This interface provides access to HTTP method handlers and basic routing
// functionality without exposing Chi-specific methods that can be problematic
// for interface abstraction.
//
// Use this interface when you need simple routing functionality and don't
// require Chi's advanced features like Route groups or sub-routers.
type BasicRouter interface {
	// HTTP method handlers for registering route handlers

	// Get registers a GET handler for the specified pattern.
	// The pattern supports Chi's URL parameter syntax: "/users/{id}"
	Get(pattern string, handler http.HandlerFunc)

	// Post registers a POST handler for the specified pattern.
	Post(pattern string, handler http.HandlerFunc)

	// Put registers a PUT handler for the specified pattern.
	Put(pattern string, handler http.HandlerFunc)

	// Delete registers a DELETE handler for the specified pattern.
	Delete(pattern string, handler http.HandlerFunc)

	// Patch registers a PATCH handler for the specified pattern.
	Patch(pattern string, handler http.HandlerFunc)

	// Head registers a HEAD handler for the specified pattern.
	Head(pattern string, handler http.HandlerFunc)

	// Options registers an OPTIONS handler for the specified pattern.
	Options(pattern string, handler http.HandlerFunc)

	// Generic handlers for registering any HTTP handler

	// Handle registers a generic HTTP handler for the specified pattern.
	// Use this when you need to handle multiple HTTP methods in one handler
	// or when working with existing http.Handler implementations.
	Handle(pattern string, handler http.Handler)

	// HandleFunc registers a generic HTTP handler function for the specified pattern.
	HandleFunc(pattern string, handler http.HandlerFunc)

	// Mounting and middleware support

	// Mount attaches another http.Handler at the specified pattern.
	// This is useful for mounting sub-applications or third-party handlers.
	// The mounted handler will receive requests with the mount pattern stripped.
	Mount(pattern string, handler http.Handler)

	// Use appends one or more middleware functions to the middleware chain.
	// Middleware is applied in the order it's added and affects all routes
	// registered after the middleware is added.
	Use(middlewares ...func(http.Handler) http.Handler)

	// HTTP handler interface

	// ServeHTTP implements the http.Handler interface, allowing the router
	// to be used directly as an HTTP handler or mounted in other routers.
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Router extends BasicRouter with Chi's full router interface.
// This interface provides access to all Chi router functionality including
// Route groups, sub-routers, and advanced routing features.
//
// Use this interface when you need Chi's advanced features like:
//   - Route groups with shared middleware
//   - Sub-routers with isolated middleware stacks
//   - Advanced routing patterns and matching
type Router interface {
	BasicRouter
	chi.Router // Embed Chi's actual Router interface for full functionality
}

// RouterService is an alias for BasicRouter.
// This provides a convenient service name for dependency injection
// when modules only need basic routing functionality.
type RouterService = BasicRouter
