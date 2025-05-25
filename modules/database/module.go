package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// Module name constant
const Name = "database"

// Module represents the database module
type Module struct {
	config      *Config
	connections map[string]*sql.DB
}

// NewModule creates a new database module
func NewModule() *Module {
	return &Module{
		connections: make(map[string]*sql.DB),
	}
}

// Name returns the name of the module
func (m *Module) Name() string {
	return Name
}

// RegisterConfig registers the module's configuration structure
func (m *Module) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &Config{
		Default:     "default",
		Connections: make(map[string]ConnectionConfig),
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the database module
func (m *Module) Init(app modular.Application) error {
	// Get the configuration
	provider, err := app.GetConfigSection("database")
	if err != nil {
		return fmt.Errorf("failed to get config section: %w", err)
	}

	// Get the actual config
	cfg, ok := provider.GetConfig().(*Config)
	if !ok {
		return fmt.Errorf("invalid config type for database module")
	}

	m.config = cfg

	// Initialize connections
	if err := m.initializeConnections(); err != nil {
		return fmt.Errorf("failed to initialize database connections: %w", err)
	}

	// Register services
	if err := m.registerServices(app); err != nil {
		return fmt.Errorf("failed to register database services: %w", err)
	}

	return nil
}

// Start starts the database module
func (m *Module) Start(ctx context.Context) error {
	// Test connections to make sure they're still alive
	for name, db := range m.connections {
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("failed to ping database connection '%s': %w", name, err)
		}
	}
	return nil
}

// Stop stops the database module
func (m *Module) Stop(ctx context.Context) error {
	for name, db := range m.connections {
		if err := db.Close(); err != nil {
			return fmt.Errorf("failed to close database connection '%s': %w", name, err)
		}
	}
	m.connections = make(map[string]*sql.DB)
	return nil
}

// Dependencies returns the names of modules this module depends on
func (m *Module) Dependencies() []string {
	return nil // No dependencies
}

// ProvidesServices declares services provided by this module
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        "database.service",
			Description: "Default database service",
			Instance:    m,
		},
		{
			Name:        "database.manager",
			Description: "Database connection manager",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module
func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil // No required services
}

// GetConnection returns a database connection by name
func (m *Module) GetConnection(name string) (*sql.DB, bool) {
	if db, exists := m.connections[name]; exists {
		return db, true
	}
	return nil, false
}

// GetDefaultConnection returns the default database connection
func (m *Module) GetDefaultConnection() *sql.DB {
	if m.config == nil || m.config.Default == "" {
		return nil
	}

	if db, exists := m.connections[m.config.Default]; exists {
		return db
	}

	// If default connection name doesn't exist, try to return any available connection
	for _, db := range m.connections {
		return db
	}

	return nil
}

// GetConnections returns a list of all available connection names
func (m *Module) GetConnections() []string {
	connections := make([]string, 0, len(m.connections))
	for name := range m.connections {
		connections = append(connections, name)
	}
	return connections
}

// initializeConnections initializes the database connections based on the module's configuration
func (m *Module) initializeConnections() error {
	// Initialize database connections
	if len(m.config.Connections) > 0 {
		for name, connConfig := range m.config.Connections {
			if connConfig.Driver == "" {
				return fmt.Errorf("database connection '%s' missing driver", name)
			}
			if connConfig.DSN == "" {
				return fmt.Errorf("database connection '%s' missing DSN", name)
			}

			// Create the database service and connect
			dbService, err := NewDatabaseService(connConfig)
			if err != nil {
				return fmt.Errorf("failed to create database service for '%s': %w", name, err)
			}

			if err := dbService.Connect(); err != nil {
				return fmt.Errorf("failed to connect to database '%s': %w", name, err)
			}

			m.connections[name] = dbService.DB()
		}
	}

	return nil
}

// registerServices registers the database services with the application
func (m *Module) registerServices(app modular.Application) error {
	// Services are already registered in ProvidesServices, this is just a placeholder
	return nil
}
