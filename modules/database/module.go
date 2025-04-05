package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/GoCodeAlone/modular"
)

// Define static errors
var (
	ErrDefaultConnectionNotFound = errors.New("default database connection not found in configuration")
)

// Module provides database connectivity for modular applications
type Module struct {
	config       *Config
	dbService    DatabaseService
	connections  map[string]DatabaseService
	defaultDBKey string
}

// NewModule creates a new database module
func NewModule() *Module {
	return &Module{
		connections: make(map[string]DatabaseService),
	}
}

// RegisterConfig registers database configuration
func (m *Module) RegisterConfig(app modular.Application) error {
	m.config = &Config{
		Connections: make(map[string]ConnectionConfig),
		Default:     "default",
	}
	app.RegisterConfigSection("database", modular.NewStdConfigProvider(m.config))

	return nil
}

// Init initializes the database connections
func (m *Module) Init(app modular.Application) error {
	if len(m.config.Connections) == 0 {
		app.Logger().Warn("No database connections configured")
		return nil
	}

	m.defaultDBKey = m.config.Default

	// Ensure default connection exists
	if _, exists := m.config.Connections[m.defaultDBKey]; !exists {
		keys := make([]string, 0, len(m.config.Connections))
		for k := range m.config.Connections {
			keys = append(keys, k)
		}
		if len(keys) > 0 {
			m.defaultDBKey = keys[0]
		} else {
			return fmt.Errorf("%w: %s", ErrDefaultConnectionNotFound, m.config.Default)
		}
	}

	// Initialize all configured connections
	for name, connConfig := range m.config.Connections {
		dbService, err := NewDatabaseService(connConfig)
		if err != nil {
			return fmt.Errorf("failed to create database connection '%s': %w", name, err)
		}
		m.connections[name] = dbService

		// Set the default dbService for backward compatibility
		if name == m.defaultDBKey {
			m.dbService = dbService
		}
	}
	app.Logger().Info("Database connections initialized", "connections", m.connections)
	app.Logger().Info("Default database connection", "default", m.defaultDBKey, "service", m.dbService)

	for name, service := range m.connections {
		if err := service.Connect(); err != nil {
			return fmt.Errorf("failed to connect to database '%s': %w", name, err)
		}
	}

	return nil
}

// Start establishes database connections
func (m *Module) Start(ctx context.Context) error {
	return nil
}

// Stop closes all database connections
func (m *Module) Stop(ctx context.Context) error {
	var lastErr error
	for name, service := range m.connections {
		if err := service.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close database connection '%s': %w", name, err)
		}
	}
	return lastErr
}

// Name returns the module name
func (m *Module) Name() string {
	return "database"
}

// ProvidesServices returns services provided by this module
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	providers := []modular.ServiceProvider{
		{
			Name:        "database.service",
			Description: "Default database service",
			Instance:    m.dbService,
		},
		{
			Name:        "database.manager",
			Description: "Database connection manager",
			Instance:    m,
		},
	}

	// Add named database services
	for name, service := range m.connections {
		providers = append(providers, modular.ServiceProvider{
			Name:        fmt.Sprintf("database.service.%s", name),
			Description: fmt.Sprintf("Database service for connection '%s'", name),
			Instance:    service,
		})
	}

	return providers
}

// RequiresServices returns services required by this module
func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil
}

// GetConnection returns a database service by name
func (m *Module) GetConnection(name string) (DatabaseService, bool) {
	service, exists := m.connections[name]
	return service, exists
}

// GetDefaultConnection returns the default database service
func (m *Module) GetDefaultConnection() DatabaseService {
	return m.dbService
}

// GetConnections returns all configured database connections
func (m *Module) GetConnections() map[string]DatabaseService {
	return m.connections
}
