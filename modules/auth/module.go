// Package auth provides authentication and authorization functionality for modular applications.
// This module supports JWT tokens, session management, and OAuth2 flows.
//
// The auth module provides:
//   - User authentication with configurable stores
//   - JWT token generation and validation
//   - Session management with configurable backends
//   - OAuth2 integration support
//   - Password hashing and validation
//
// Usage:
//   app.RegisterModule(auth.NewModule())
//
// The module registers an "auth" service that implements the AuthService interface,
// providing methods for user login, token validation, and session management.
//
// Configuration:
//   The module requires an "auth" configuration section with JWT secrets,
//   session settings, and OAuth2 configuration.
package auth

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

const (
	// ServiceName is the name used to register the auth service.
	// Other modules can reference this service by this name for dependency injection.
	ServiceName = "auth"
)

// Module implements the modular.Module interface for authentication.
// It provides comprehensive authentication and authorization functionality
// including JWT tokens, sessions, and OAuth2 support.
//
// The module is designed to work with pluggable stores for users and sessions,
// defaulting to in-memory implementations if external stores are not provided.
type Module struct {
	config  *Config
	service *Service
	logger  modular.Logger
}

// NewModule creates a new authentication module.
// The returned module must be registered with the application before use.
//
// Example:
//   authModule := auth.NewModule()
//   app.RegisterModule(authModule)
func NewModule() modular.Module {
	return &Module{}
}

// Name returns the module name.
// This name is used for dependency resolution and service registration.
func (m *Module) Name() string {
	return "auth"
}

// RegisterConfig registers the module's configuration requirements.
// This method sets up the configuration structure for the auth module,
// allowing the application to load authentication-related settings.
//
// The auth module expects configuration for:
//   - JWT secret keys and token expiration
//   - Session configuration (timeouts, secure flags)
//   - OAuth2 provider settings
//   - Password policy settings
func (m *Module) RegisterConfig(app modular.Application) error {
	m.config = &Config{}
	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(m.config))
	return nil
}

// Init initializes the authentication module.
// This method validates the configuration and prepares the module for use.
// The actual service creation happens in the Constructor method to support
// dependency injection of user and session stores.
func (m *Module) Init(app modular.Application) error {
	m.logger = app.Logger()

	// Validate configuration
	if err := m.config.Validate(); err != nil {
		return fmt.Errorf("auth module configuration validation failed: %w", err)
	}

	m.logger.Info("Authentication module initialized", "module", m.Name())
	return nil
}

// Start starts the authentication module.
// Currently the auth module doesn't require any startup operations,
// but this method is available for future enhancements like background
// token cleanup or session maintenance tasks.
func (m *Module) Start(ctx context.Context) error {
	m.logger.Info("Authentication module started", "module", m.Name())
	return nil
}

// Stop stops the authentication module.
// This method can be used for cleanup operations like closing database
// connections or stopping background tasks when they are added in the future.
func (m *Module) Stop(ctx context.Context) error {
	m.logger.Info("Authentication module stopped", "module", m.Name())
	return nil
}

// Dependencies returns the module dependencies.
// The auth module has no required module dependencies, making it suitable
// for use as a foundation module that other modules can depend on.
func (m *Module) Dependencies() []string {
	return nil // No explicit module dependencies
}

// ProvidesServices returns the services provided by this module.
// The auth module provides an authentication service that implements
// the AuthService interface, offering methods for user login, token
// validation, and session management.
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Authentication service providing JWT, sessions, and OAuth2 support",
			Instance:    m.service,
		},
	}
}

// RequiresServices returns the services required by this module.
// The auth module can optionally use external user and session stores.
// If these services are not provided, the module will fall back to
// in-memory implementations suitable for development and testing.
//
// Optional services:
//   - user_store: Implementation of UserStore interface for persistent user data
//   - session_store: Implementation of SessionStore interface for session persistence
func (m *Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:     "user_store",
			Required: false, // Optional - will use in-memory store if not provided
		},
		{
			Name:     "session_store",
			Required: false, // Optional - will use in-memory store if not provided
		},
	}
}

// Constructor provides dependency injection for the module.
// This method creates the authentication service with injected dependencies,
// using fallback implementations for optional services that aren't provided.
//
// The constructor pattern allows the module to be reconstructed with proper
// dependency injection after all required services have been resolved.
//
// Dependencies resolved:
//   - user_store: External user storage (falls back to memory store)
//   - session_store: External session storage (falls back to memory store)
func (m *Module) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get user store (use mock if not provided)
		var userStore UserStore
		if us, ok := services["user_store"]; ok {
			if userStoreImpl, ok := us.(UserStore); ok {
				userStore = userStoreImpl
			} else {
				return nil, fmt.Errorf("user_store service does not implement UserStore interface")
			}
		} else {
			userStore = NewMemoryUserStore()
		}

		// Get session store (use mock if not provided)
		var sessionStore SessionStore
		if ss, ok := services["session_store"]; ok {
			if sessionStoreImpl, ok := ss.(SessionStore); ok {
				sessionStore = sessionStoreImpl
			} else {
				return nil, fmt.Errorf("session_store service does not implement SessionStore interface")
			}
		} else {
			sessionStore = NewMemorySessionStore()
		}

		// Create the auth service
		m.service = NewService(m.config, userStore, sessionStore)

		return m, nil
	}
}
