# chimux Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/chimux.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/chimux)

A module for the [Modular](https://github.com/GoCodeAlone/modular) framework.

## Overview

The chimux module provides a powerful HTTP router and middleware system for Modular applications by integrating the popular [go-chi](https://github.com/go-chi/chi) router. This module allows Modular applications to easily set up and configure HTTP routing with middleware support, all while maintaining the modular architecture and configuration management that Modular offers.

## Features

* Full integration of go-chi router within the Modular framework
* Configurable middleware stack with pre-defined middleware options 
* Easy route registration through service interfaces
* Support for RESTful resource patterns
* Mount handlers at specific path prefixes
* Configurable CORS settings
* Timeout management for request handling
* Base path configuration for all routes

## Installation

```go
go get github.com/GoCodeAlone/modular/modules/chimux@v1.0.0
```

## Usage

```go
package main

import (
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
	)

	// Register the chimux module
	app.RegisterModule(chimux.NewChimuxModule())
	
	// Register your API module that will use the router
	app.RegisterModule(NewAPIModule())

	// Run the application
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

// APIModule that uses the chimux router
type APIModule struct {
	router chimux.RouterService
}

func NewAPIModule() modular.Module {
	return &APIModule{}
}

func (m *APIModule) Name() string {
	return "api"
}

func (m *APIModule) Dependencies() []string {
	return []string{"chimux"} // Depend on chimux module
}

func (m *APIModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:     "chimux.router",
			Required: true,
		},
	}
}

func (m *APIModule) Init(app modular.Application) error {
	// Get the router service
	if err := app.GetService("chimux.router", &m.router); err != nil {
		return err
	}
	
	// Register routes
	m.router.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	
	m.router.Route("/api/users", func(r chimux.Router) {
		r.Get("/", m.listUsers)
		r.Post("/", m.createUser)
		r.Route("/{id}", func(r chimux.Router) {
			r.Get("/", m.getUser)
			r.Put("/", m.updateUser)
			r.Delete("/", m.deleteUser)
		})
	})
	
	return nil
}

// Other required module methods...
```
## Configuration

The chimux module supports the following configuration options:

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| AllowedOrigins | []string | No | ["*"] | List of allowed origins for CORS requests. |
| AllowedMethods | []string | No | ["GET","POST","PUT","DELETE","OPTIONS"] | List of allowed HTTP methods. |
| AllowedHeaders | []string | No | ["Origin","Accept","Content-Type","X-Requested-With","Authorization"] | List of allowed request headers. |
| AllowCredentials | bool | No | false | Allow credentials in CORS requests. |
| MaxAge | int | No | 300 | Maximum age for CORS preflight cache in seconds. |
| Timeout | int | No | 60000 | Default request timeout in milliseconds. |
| BasePath | string | No | - | A base path prefix for all routes registered through this module. |
| EnabledMiddleware | []string | No | ["Heartbeat","RequestID","RealIP","Logger","Recoverer"] | List of middleware to enable by default. |

### Example Configuration

```yaml
# config.yaml
chimux:
  allowedorigins: ["*"]
  allowedmethods: ["GET","POST","PUT","DELETE","OPTIONS"]
  allowedheaders: ["Origin","Accept","Content-Type","X-Requested-With","Authorization"]
  allowcredentials: false
  maxage: 300
  timeout: 60000
  basepath: "/api"
  enabledmiddleware: ["Heartbeat", "RequestID", "RealIP", "Logger", "Recoverer"]
```

## Middleware Configuration

chimux supports two approaches for configuring middleware:

### 1. Configuration-based middleware

Define which built-in middleware to enable through configuration:

```yaml
chimux:
  enabledmiddleware: 
    - "RequestID"
    - "RealIP"
    - "Logger"
    - "Recoverer"
    - "StripSlashes"
    - "Timeout"
```

### 2. Programmatic middleware registration

For custom middleware, you can register it during module initialization or by implementing the `MiddlewareProvider` interface:

```go
// Custom middleware provider module
type AuthMiddlewareModule struct {
    // module implementation
}

func (m *AuthMiddlewareModule) ProvidesServices() []modular.ServiceProvider {
    return []modular.ServiceProvider{
        {
            Name: "auth.middleware",
            Instance: chimux.MiddlewareProvider(func() []chimux.Middleware {
                return []chimux.Middleware{
                    m.jwtAuthMiddleware,
                    m.roleCheckerMiddleware,
                }
            }),
        },
    }
}

// Define your custom middleware
func (m *AuthMiddlewareModule) jwtAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // JWT authentication logic
        next.ServeHTTP(w, r)
    })
}
```

The chimux module will automatically discover and use any registered `MiddlewareProvider` services using interface-based service matching.

## Advanced Usage

### Route Pattern Matching & Dynamic Segment Mismatches

The underlying Chi router matches the *pattern shape* – a registered route with a
dynamic segment (e.g. `/api/users/{id}`) matches `/api/users/123` as expected, but a
request to `/api/users/` (trailing slash, missing segment) or `/api/users` (no trailing
slash, missing segment) will **not** invoke that handler. This is intentional: Chi treats
`/api/users` and `/api/users/` as distinct from `/api/users/{id}` to avoid accidental
shadowing and ambiguous parameter extraction.

If you want both collection and entity semantics, register both patterns explicitly:

```go
router.Route("/api/users", func(r chimux.Router) {
    r.Get("/", listUsers)        // GET /api/users
    r.Post("/", createUser)      // POST /api/users
    r.Route("/{id}", func(r chimux.Router) { // GET /api/users/{id}
        r.Get("/", getUser)     // (Chi normalizes without extra segment; trailing slash optional when calling)
        r.Put("/", updateUser)
        r.Delete("/", deleteUser)
    })
})
```

For optional trailing segments, prefer explicit duplication instead of relying on
middleware redirects. Keeping patterns explicit makes route introspection, dynamic
enable/disable operations, and emitted routing events deterministic.

### Adding custom middleware to specific routes

```go
func (m *APIModule) Init(app modular.Application) error {
    // Get the router service
    err := app.GetService("chimux.router", &m.router)
    if err != nil {
        return err
    }
    
    // Create middleware
    adminOnly := func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Check if user is admin
            if isAdmin := checkAdmin(r); !isAdmin {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
    
    // Apply middleware to specific routes
    m.router.Route("/admin", func(r chimux.Router) {
        r.Use(adminOnly)
        r.Get("/dashboard", m.adminDashboard)
        r.Post("/users", m.adminCreateUser)
    })
    
    return nil
}
```

### Accessing the underlying chi.Router

If needed, you can access the underlying chi.Router for advanced functionality:

```go
func (m *APIModule) Init(app modular.Application) error {
    var router chimux.ChiRouterService
    if err := app.GetService("chimux.router", &router); err != nil {
        return err
    }
    
    // Access the underlying chi.Router
    chiRouter := router.ChiRouter()
    
    // Use chi-specific features
    chiRouter.Mount("/legacy", legacyHandler)
    
    return nil
}
```

## License

[MIT License](LICENSE)
