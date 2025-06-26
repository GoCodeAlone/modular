package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/GoCodeAlone/modular"
)

// Module name constant
const Name = "database"

// lazyDefaultService is a wrapper that lazily resolves the default database service
type lazyDefaultService struct {
	module *Module
}

func (l *lazyDefaultService) Connect() error {
	service := l.module.GetDefaultService()
	if service == nil {
		return errors.New("no default database service available")
	}
	return service.Connect()
}

func (l *lazyDefaultService) Close() error {
	service := l.module.GetDefaultService()
	if service == nil {
		return errors.New("no default database service available")
	}
	return service.Close()
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
		return errors.New("no default database service available")
	}
	return service.Ping(ctx)
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
		return nil, errors.New("no default database service available")
	}
	return service.ExecContext(ctx, query, args...)
}

func (l *lazyDefaultService) Exec(query string, args ...interface{}) (sql.Result, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.Exec(query, args...)
}

func (l *lazyDefaultService) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.PrepareContext(ctx, query)
}

func (l *lazyDefaultService) Prepare(query string) (*sql.Stmt, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.Prepare(query)
}

func (l *lazyDefaultService) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.QueryContext(ctx, query, args...)
}

func (l *lazyDefaultService) Query(query string, args ...interface{}) (*sql.Rows, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.Query(query, args...)
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
		return nil, errors.New("no default database service available")
	}
	return service.BeginTx(ctx, opts)
}

func (l *lazyDefaultService) Begin() (*sql.Tx, error) {
	service := l.module.GetDefaultService()
	if service == nil {
		return nil, errors.New("no default database service available")
	}
	return service.Begin()
}

// Module represents the database module
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

// NewModule creates a new database module
func NewModule() *Module {
	return &Module{
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
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

	// Create instance-aware config provider with database-specific prefix
	instancePrefixFunc := func(instanceKey string) string {
		return "DB_" + instanceKey + "_"
	}
	
	configProvider := modular.NewInstanceAwareConfigProvider(defaultConfig, instancePrefixFunc)
	app.RegisterConfigSection(m.Name(), configProvider)
	return nil
}

// Init initializes the database module
func (m *Module) Init(app modular.Application) error {
	// Get the configuration
	provider, err := app.GetConfigSection(m.Name())
	if err != nil {
		return fmt.Errorf("failed to get config section: %w", err)
	}

	// Get the actual config
	cfg, ok := provider.GetConfig().(*Config)
	if !ok {
		return ErrInvalidConfigType
	}

	m.config = cfg

	// Initialize connections
	if err := m.initializeConnections(); err != nil {
		return fmt.Errorf("failed to initialize database connections: %w", err)
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

// Dependencies returns the names of modules this module depends on
func (m *Module) Dependencies() []string {
	return nil // No dependencies
}

// ProvidesServices declares services provided by this module
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

// GetDefaultService returns the default database service
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

// GetService returns a database service by name
func (m *Module) GetService(name string) (DatabaseService, bool) {
	if service, exists := m.services[name]; exists {
		return service, true
	}
	return nil, false
}

// initializeConnections initializes the database connections based on the module's configuration
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
			dbService, err := NewDatabaseService(connConfig)
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
