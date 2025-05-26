package auth

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

const (
	// ServiceName is the name used to register the auth service
	ServiceName = "auth"
)

// Module implements the modular.Module interface for authentication
type Module struct {
	config  *Config
	service *Service
	logger  modular.Logger
}

// NewModule creates a new authentication module
func NewModule() modular.Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string {
	return "auth"
}

// RegisterConfig registers the module's configuration
func (m *Module) RegisterConfig(app modular.Application) error {
	m.config = &Config{}
	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(m.config))
	return nil
}

// Init initializes the authentication module
func (m *Module) Init(app modular.Application) error {
	m.logger = app.Logger()

	// Validate configuration
	if err := m.config.Validate(); err != nil {
		return fmt.Errorf("auth module configuration validation failed: %w", err)
	}

	m.logger.Info("Authentication module initialized", "module", m.Name())
	return nil
}

// Start starts the authentication module
func (m *Module) Start(ctx context.Context) error {
	m.logger.Info("Authentication module started", "module", m.Name())
	return nil
}

// Stop stops the authentication module
func (m *Module) Stop(ctx context.Context) error {
	m.logger.Info("Authentication module stopped", "module", m.Name())
	return nil
}

// Dependencies returns the module dependencies
func (m *Module) Dependencies() []string {
	return nil // No explicit module dependencies
}

// ProvidesServices returns the services provided by this module
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Authentication service providing JWT, sessions, and OAuth2 support",
			Instance:    m.service,
		},
	}
}

// RequiresServices returns the services required by this module
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

// Constructor provides dependency injection for the module
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
