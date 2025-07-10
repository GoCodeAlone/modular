// Package database provides database connectivity and management for modular applications.
// This module supports multiple database connections with different drivers and provides
// a unified interface for database operations.
//
// The database module features:
//   - Multiple database connections with configurable drivers (MySQL, PostgreSQL, SQLite, etc.)
//   - Connection pooling and health monitoring
//   - Default connection selection for simplified usage
//   - Database service abstraction for testing and mocking
//   - Instance-aware configuration for environment-specific settings
//
// Usage:
//
//	app.RegisterModule(database.NewModule())
//
// The module registers database services that provide access to sql.DB instances
// and higher-level database operations. Other modules can depend on these services
// for database access.
//
// Configuration:
//
//	The module requires a "database" configuration section with connection details
//	for each database instance, including driver, DSN, and connection pool settings.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// Static errors for err113 compliance
var (
	ErrNoDefaultService = errors.New("no default database service available")
)

// Module name constant for service registration and dependency resolution.
const Name = "database"

// lazyDefaultService is a wrapper that lazily resolves the default database service.
// This wrapper allows the database service to be registered before the actual
// database connections are established, supporting proper dependency injection
// while maintaining lazy initialization of database resources.
type lazyDefaultService struct {
	module *Module
}

func (l *lazyDefaultService) Connect() error {
	service := l.module.GetDefaultService()
	if service == nil {
		return ErrNoDefaultService
	}
	if err := service.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	return nil
}

func (l *lazyDefaultService) Close() error {
	service := l.module.GetDefaultService()
	if service == nil {
		return ErrNoDefaultService
	}
	if err := service.Close(); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}
	return nil
}

func (l *lazyDefaultService) DB() *sql.DB {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil
	}
	return service.DB()
}

func (l *lazyDefaultService) Ping(ctx context.Context) error {
	service := l.module.GetDefaultService()
	if service == nil {
		return ErrNoDefaultService
	}
	if err := service.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}
	return nil
}

func (l *lazyDefaultService) Stats() sql.DBStats {
	service := l.module.GetDefaultService()
	if service == nil {
		return sql.DBStats{}
	}
	return service.Stats()
}

func (l *lazyDefaultService) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	result, err := service.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return result, nil
}

func (l *lazyDefaultService) Exec(query string, args ...interface{}) (sql.Result, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	result, err := service.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return result, nil
}

func (l *lazyDefaultService) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	stmt, err := service.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	return stmt, nil
}

func (l *lazyDefaultService) Prepare(query string) (*sql.Stmt, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	stmt, err := service.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	return stmt, nil
}

func (l *lazyDefaultService) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	rows, err := service.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	return rows, nil
}

func (l *lazyDefaultService) Query(query string, args ...interface{}) (*sql.Rows, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	rows, err := service.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	return rows, nil
}

func (l *lazyDefaultService) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil
	}
	return service.QueryRowContext(ctx, query, args...)
}

func (l *lazyDefaultService) QueryRow(query string, args ...interface{}) *sql.Row {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil
	}
	return service.QueryRow(query, args...)
}

func (l *lazyDefaultService) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	tx, err := service.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

func (l *lazyDefaultService) Begin() (*sql.Tx, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, ErrNoDefaultService
	}
	tx, err := service.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// Module represents the database module and implements the modular.Module interface.
// It manages multiple database connections and provides services for database access.
//
// The module supports:
//   - Multiple named database connections
//   - Automatic connection health monitoring
//   - Default connection selection
//   - Service abstraction for easier testing
//   - Instance-aware configuration
type Module struct {
	config      *Config
	connections map[string]*sql.DB
	services    map[string]DatabaseService
}

var (
	ErrInvalidConfigType = errors.New("invalid config type for database module")
	ErrMissingDriver     = errors.New("database connection missing driver")
	ErrMissingDSN        = errors.New("database connection missing DSN")
)

// NewModule creates a new database module instance.
// The returned module must be registered with the application before use.
//
// Example:
//
//	dbModule := database.NewModule()
//	app.RegisterModule(dbModule)
func NewModule() *Module {
	return &Module{
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}
}

// Name returns the name of the module.
// This name is used for dependency resolution and configuration section lookup.
func (m *Module) Name() string {
	return Name
}

// RegisterConfig registers the module's configuration structure.
// The database module uses instance-aware configuration to support
// environment-specific database connection settings.
//
// Configuration structure:
//   - Default: name of the default connection to use
//   - Connections: map of connection names to their configurations
//
// Environment variables:
//
//	DB_<CONNECTION_NAME>_DRIVER, DB_<CONNECTION_NAME>_DSN, etc.
func (m *Module) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &Config{
		Default:     "default",
		Connections: make(map[string]*ConnectionConfig),
	}

	// Create instance-aware config provider with database-specific prefix
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}

	configProvider := modular.NewInstanceAwareConfigProvider(defaultConfig, instancePrefixFunc)
	app.RegisterConfigSection(m.Name(), configProvider)
	return nil
}

// Init initializes the database module and establishes database connections.
// This method loads the configuration, validates connection settings,
// and establishes connections to all configured databases.
//
// Initialization process:
//  1. Load configuration from the "database" section
//  2. Validate connection configurations
//  3. Create database services for each connection
//  4. Test initial connectivity
func (m *Module) Init(app modular.Application) error {
	// Get the configuration
	provider, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section: %w", err)
	}

	// Get the actual config - it may be from an instance-aware provider
	var cfg *Config
	if iaProvider, ok := provider.(*modular.InstanceAwareConfigProvider); ok {
		// For instance-aware providers, get the fed config
		cfg, ok = iaProvider.GetConfig().(*Config)
		if !ok {
			return ErrInvalidConfigType
		}
	} else {
		// For standard providers
		cfg, ok = provider.GetConfig().(*Config)
		if !ok {
			return ErrInvalidConfigType
		}
	}

	m.config = cfg

	// Initialize connections
	if err := m.initializeConnections(); err != nil {
		return fmt.Errorf("failed to initialize database connections: %w", err)
	}

	return nil
}

// Start starts the database module and verifies connectivity.
// This method performs health checks on all database connections
// to ensure they are ready for use by other modules.
func (m *Module) Start(ctx context.Context) error {
	// Test connections to make sure they're still alive
	for name, db := range m.connections {
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("failed to ping database connection '%s': %w", name, err)
		}
	}
	return nil
}

// Stop stops the database module and closes all connections.
// This method gracefully closes all database connections and services,
// ensuring proper cleanup during application shutdown.
func (m *Module) Stop(ctx context.Context) error {
	// Close all database services
	for name, service := range m.services {
		if err := service.Close(); err != nil {
			return fmt.Errorf("failed to close database service '%s': %w", name, err)
		}
	}

	// Clear the maps
	m.connections = make(map[string]*sql.DB)
	m.services = make(map[string]DatabaseService)
	return nil
}

// Dependencies returns the names of modules this module depends on.
// The database module has no dependencies, making it suitable as a
// foundation module that other modules can depend on.
func (m *Module) Dependencies() []string {
	return nil // No dependencies
}

// ProvidesServices declares services provided by this module.
// The database module provides:
//   - database.manager: Module instance for direct database management
//   - database.service: Default database service for convenient access
//
// Other modules can depend on these services to access database functionality.
func (m *Module) ProvidesServices() []modular.ServiceProvider {
	providers := []modular.ServiceProvider{
		{
			Name:        "database.manager",
			Description: "Database connection manager",
			Instance:    m,
		},
		{
			Name:        "database.service",
			Description: "Default database service",
			Instance:    &lazyDefaultService{module: m}, // Lazy wrapper that resolves at runtime
		},
	}

	return providers
}

// RequiresServices declares services required by this module.
// The database module is self-contained and doesn't require
// services from other modules.
func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil // No required services
}

// GetConnection returns a database connection by name.
// This method allows access to specific named database connections
// that were configured in the module's configuration.
//
// Example:
//
//	if db, exists := dbModule.GetConnection("primary"); exists {
//	    // Use the primary database connection
//	}
func (m *Module) GetConnection(name string) (*sql.DB, bool) {
	if db, exists := m.connections[name]; exists {
		return db, true
	}
	return nil, false
}

// GetDefaultConnection returns the default database connection.
// The default connection is determined by the "default" field in the
// configuration. If no default is configured or the named connection
// doesn't exist, this method will return any available connection.
//
// Returns nil if no connections are available.
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

// GetConnections returns a list of all available connection names.
// This is useful for discovery and diagnostic purposes.
func (m *Module) GetConnections() []string {
	connections := make([]string, 0, len(m.connections))
	for name := range m.connections {
		connections = append(connections, name)
	}
	return connections
}

// GetDefaultService returns the default database service.
// Similar to GetDefaultConnection, but returns a DatabaseService
// interface that provides additional functionality beyond the raw sql.DB.
func (m *Module) GetDefaultService() DatabaseService {
	if m.config == nil || m.config.Default == "" {
		return nil
	}

	if service, exists := m.services[m.config.Default]; exists {
		return service
	}

	// If default connection name doesn't exist, try to return any available service
	for _, service := range m.services {
		return service
	}

	return nil
}

// GetService returns a database service by name.
// Database services provide a higher-level interface than raw database
// connections, including connection management and additional utilities.
func (m *Module) GetService(name string) (DatabaseService, bool) {
	if service, exists := m.services[name]; exists {
		return service, true
	}
	return nil, false
}

// initializeConnections initializes database connections based on the module's configuration.
// This method processes each configured connection, creates database services,
// and establishes initial connectivity to validate the configuration.
func (m *Module) initializeConnections() error {
	// Initialize database connections
	if len(m.config.Connections) > 0 {
		for name, connConfig := range m.config.Connections {
			if connConfig.Driver == "" {
				return ErrMissingDriver
			}
			if connConfig.DSN == "" {
				return ErrMissingDSN
			}

			// Create the database service and connect
			dbService, err := NewDatabaseService(*connConfig)
			if err != nil {
				return fmt.Errorf("failed to create database service for '%s': %w", name, err)
			}

			if err := dbService.Connect(); err != nil {
				return fmt.Errorf("failed to connect to database '%s': %w", name, err)
			}

			m.connections[name] = dbService.DB()
			m.services[name] = dbService
		}
	}

	return nil
}
