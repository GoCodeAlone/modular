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
