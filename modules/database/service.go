package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Define static errors
var (
	ErrEmptyDriver          = errors.New("database driver cannot be empty")
	ErrEmptyDSN             = errors.New("database connection string (DSN) cannot be empty")
	ErrDatabaseNotConnected = errors.New("database not connected")
)

// Constants for database service
const (
	// DefaultConnectionTimeout is the default timeout for database connection tests
	DefaultConnectionTimeout = 5 * time.Second
)

// DatabaseService defines the operations that can be performed with a database
type DatabaseService interface {
	// Connect establishes the database connection
	Connect() error

	// Close closes the database connection
	Close() error

	// DB returns the underlying database connection
	DB() *sql.DB

	// Ping verifies the database connection is still alive
	Ping(ctx context.Context) error

	// Stats returns database statistics
	Stats() sql.DBStats

	// ExecContext executes a query without returning any rows
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Exec executes a query without returning any rows (using default context)
	Exec(query string, args ...interface{}) (sql.Result, error)

	// ExecuteContext executes a query without returning any rows (alias for ExecContext)
	// Kept for backwards compatibility with earlier API docs/tests
	ExecuteContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Execute executes a query without returning any rows (alias for Exec)
	// Kept for backwards compatibility with earlier API docs/tests
	Execute(query string, args ...interface{}) (sql.Result, error)

	// PrepareContext prepares a statement for execution
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)

	// Prepare prepares a statement for execution (using default context)
	Prepare(query string) (*sql.Stmt, error)

	// QueryContext executes a query that returns rows
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)

	// Query executes a query that returns rows (using default context)
	Query(query string, args ...interface{}) (*sql.Rows, error)

	// QueryRowContext executes a query that returns a single row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	// QueryRow executes a query that returns a single row (using default context)
	QueryRow(query string, args ...interface{}) *sql.Row

	// BeginTx starts a transaction
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Begin starts a transaction with default options
	Begin() (*sql.Tx, error)

	// CommitTransaction commits a transaction and emits appropriate events
	CommitTransaction(ctx context.Context, tx *sql.Tx) error

	// RollbackTransaction rolls back a transaction and emits appropriate events
	RollbackTransaction(ctx context.Context, tx *sql.Tx) error

	// Migration operations
	// RunMigration executes a database migration
	RunMigration(ctx context.Context, migration Migration) error

	// GetAppliedMigrations returns a list of applied migration IDs
	GetAppliedMigrations(ctx context.Context) ([]string, error)

	// CreateMigrationsTable ensures the migrations tracking table exists
	CreateMigrationsTable(ctx context.Context) error

	// SetEventEmitter sets the event emitter for migration events
	SetEventEmitter(emitter EventEmitter)
}

// databaseServiceImpl implements the DatabaseService interface
type databaseServiceImpl struct {
	config           ConnectionConfig
	db               *sql.DB
	awsTokenProvider IAMTokenProvider
	migrationService MigrationService
	eventEmitter     EventEmitter
	logger           modular.Logger // Logger service for error reporting
	ctx              context.Context
	cancel           context.CancelFunc
	endpoint         string       // Store endpoint for reconnection
	connMutex        sync.RWMutex // Protect database connection during recreation
}

// NewDatabaseService creates a new database service from configuration
func NewDatabaseService(config ConnectionConfig, logger modular.Logger) (DatabaseService, error) {
	if config.Driver == "" {
		return nil, ErrEmptyDriver
	}

	if config.DSN == "" {
		return nil, ErrEmptyDSN
	}

	ctx, cancel := context.WithCancel(context.Background())
	service := &databaseServiceImpl{
		config: config,
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}

	// Initialize AWS IAM token provider if enabled
	if config.AWSIAMAuth != nil && config.AWSIAMAuth.Enabled {
		tokenProvider, err := NewAWSIAMTokenProvider(config.AWSIAMAuth)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create AWS IAM token provider: %w", err)
		}
		service.awsTokenProvider = tokenProvider
	}

	return service, nil
}

func (s *databaseServiceImpl) Connect() error {
	dsn := s.config.DSN

	// If AWS IAM authentication is enabled, get the token and update the DSN
	if s.awsTokenProvider != nil {
		var err error
		dsn, err = s.awsTokenProvider.BuildDSNWithIAMToken(s.ctx, s.config.DSN)
		if err != nil {
			return fmt.Errorf("failed to build DSN with IAM token: %w", err)
		}

		// Extract and store endpoint for token refresh
		s.endpoint, err = extractEndpointFromDSN(s.config.DSN)
		if err != nil {
			return fmt.Errorf("failed to extract endpoint for token refresh: %w", err)
		}

		// Set up token refresh callback to recreate connections when tokens are refreshed
		s.awsTokenProvider.SetTokenRefreshCallback(s.onTokenRefresh)

		// Start background token refresh
		s.awsTokenProvider.StartTokenRefresh(s.ctx, s.endpoint)
	} else {
		// Only preprocess when NOT using AWS IAM auth (since AWS IAM auth does its own preprocessing)
		var err error
		dsn, err = preprocessDSNForParsing(dsn)
		if err != nil {
			return fmt.Errorf("failed to preprocess DSN: %w", err)
		}
	}

	db, err := sql.Open(s.config.Driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	if s.config.MaxOpenConnections > 0 {
		db.SetMaxOpenConns(s.config.MaxOpenConnections)
	}
	if s.config.MaxIdleConnections > 0 {
		db.SetMaxIdleConns(s.config.MaxIdleConnections)
	}
	if s.config.ConnectionMaxLifetime > 0 {
		db.SetConnMaxLifetime(s.config.ConnectionMaxLifetime)
	}
	if s.config.ConnectionMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(s.config.ConnectionMaxIdleTime)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("failed to ping database and close connection: %w", err)
		}
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.db = db

	// Initialize migration service after successful connection
	if s.eventEmitter != nil {
		s.migrationService = NewMigrationService(s.db, s.eventEmitter)
	}

	return nil
}

func (s *databaseServiceImpl) Close() error {
	// Stop AWS token refresh if running
	if s.awsTokenProvider != nil {
		s.awsTokenProvider.StopTokenRefresh()
	}

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Close database connection
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return fmt.Errorf("closing database connection: %w", err)
		}
	}
	return nil
}

func (s *databaseServiceImpl) DB() *sql.DB {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return s.db
}

func (s *databaseServiceImpl) Ping(ctx context.Context) error {
	if s.db == nil {
		return ErrDatabaseNotConnected
	}
	err := s.db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	return nil
}

func (s *databaseServiceImpl) Stats() sql.DBStats {
	if s.db == nil {
		return sql.DBStats{}
	}
	return s.db.Stats()
}

func (s *databaseServiceImpl) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	return result, nil
}

func (s *databaseServiceImpl) Exec(query string, args ...interface{}) (sql.Result, error) {
	return s.ExecContext(context.Background(), query, args...)
}

// ExecuteContext is a backward-compatible alias for ExecContext
func (s *databaseServiceImpl) ExecuteContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.ExecContext(ctx, query, args...)
}

// Execute is a backward-compatible alias for Exec
func (s *databaseServiceImpl) Execute(query string, args ...interface{}) (sql.Result, error) {
	return s.Exec(query, args...)
}

func (s *databaseServiceImpl) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying database: %w", err)
	}
	return rows, nil
}

func (s *databaseServiceImpl) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.QueryContext(context.Background(), query, args...)
}

func (s *databaseServiceImpl) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if s.db == nil {
		return nil
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *databaseServiceImpl) QueryRow(query string, args ...interface{}) *sql.Row {
	return s.QueryRowContext(context.Background(), query, args...)
}

func (s *databaseServiceImpl) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("beginning database transaction: %w", err)
	}
	return tx, nil
}

func (s *databaseServiceImpl) Begin() (*sql.Tx, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("beginning database transaction: %w", err)
	}
	return tx, nil
}

// CommitTransaction commits a transaction and emits appropriate events
func (s *databaseServiceImpl) CommitTransaction(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return ErrTransactionNil
	}

	startTime := time.Now()
	err := tx.Commit()
	duration := time.Since(startTime)

	// Emit transaction committed event
	if s.eventEmitter != nil {
		go func() {
			event := modular.NewCloudEvent(EventTypeTransactionCommitted, "database-service", map[string]interface{}{
				"connection":   "default",
				"committed_at": startTime.Format(time.RFC3339),
				"duration_ms":  duration.Milliseconds(),
			}, nil)

			if emitErr := s.eventEmitter.EmitEvent(ctx, event); emitErr != nil {
				log.Printf("Failed to emit transaction committed event: %v", emitErr)
			}
		}()
	}

	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RollbackTransaction rolls back a transaction and emits appropriate events
func (s *databaseServiceImpl) RollbackTransaction(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return ErrTransactionNil
	}

	startTime := time.Now()
	err := tx.Rollback()
	duration := time.Since(startTime)

	// Emit transaction rolled back event
	if s.eventEmitter != nil {
		go func() {
			event := modular.NewCloudEvent(EventTypeTransactionRolledBack, "database-service", map[string]interface{}{
				"connection":     "default",
				"rolled_back_at": startTime.Format(time.RFC3339),
				"duration_ms":    duration.Milliseconds(),
				"reason":         "manual rollback",
			}, nil)

			if emitErr := s.eventEmitter.EmitEvent(ctx, event); emitErr != nil {
				log.Printf("Failed to emit transaction rolled back event: %v", emitErr)
			}
		}()
	}

	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	return nil
}

func (s *databaseServiceImpl) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	return stmt, nil
}

func (s *databaseServiceImpl) Prepare(query string) (*sql.Stmt, error) {
	return s.PrepareContext(context.Background(), query)
}

// SetEventEmitter sets the event emitter for migration events
func (s *databaseServiceImpl) SetEventEmitter(emitter EventEmitter) {
	s.eventEmitter = emitter
	// Re-initialize migration service if database is already connected
	if s.db != nil {
		s.migrationService = NewMigrationService(s.db, s.eventEmitter)
	}
}

// Migration methods - delegate to migration service

func (s *databaseServiceImpl) RunMigration(ctx context.Context, migration Migration) error {
	if s.migrationService == nil {
		return ErrMigrationServiceNotInitialized
	}
	err := s.migrationService.RunMigration(ctx, migration)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}
	return nil
}

func (s *databaseServiceImpl) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	if s.migrationService == nil {
		return nil, ErrMigrationServiceNotInitialized
	}
	migrations, err := s.migrationService.GetAppliedMigrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	return migrations, nil
}

func (s *databaseServiceImpl) CreateMigrationsTable(ctx context.Context) error {
	if s.migrationService == nil {
		return ErrMigrationServiceNotInitialized
	}
	if err := s.migrationService.CreateMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	return nil
}

// onTokenRefresh is called when IAM token is refreshed to recreate database connections
func (s *databaseServiceImpl) onTokenRefresh(newToken string, endpoint string) {
	// Recreate database connection with new token
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.db == nil {
		return // Connection already closed
	}

	// Close existing connections to force pool refresh
	oldDB := s.db

	// Build new DSN with refreshed token
	newDSN, err := s.awsTokenProvider.BuildDSNWithIAMToken(s.ctx, s.config.DSN)
	if err != nil {
		// Log error but don't crash the application
		s.logger.Error("Failed to build DSN with refreshed IAM token", "error", err, "endpoint", endpoint)
		return
	}

	// Create new database connection
	newDB, err := sql.Open(s.config.Driver, newDSN)
	if err != nil {
		s.logger.Error("Failed to create new database connection with refreshed token", "error", err, "endpoint", endpoint)
		return
	}

	// Configure connection pool settings
	if s.config.MaxOpenConnections > 0 {
		newDB.SetMaxOpenConns(s.config.MaxOpenConnections)
	}
	if s.config.MaxIdleConnections > 0 {
		newDB.SetMaxIdleConns(s.config.MaxIdleConnections)
	}
	if s.config.ConnectionMaxLifetime > 0 {
		newDB.SetConnMaxLifetime(s.config.ConnectionMaxLifetime)
	}
	if s.config.ConnectionMaxIdleTime > 0 {
		newDB.SetConnMaxIdleTime(s.config.ConnectionMaxIdleTime)
	}

	// Test the new connection with a timeout
	timeout := DefaultConnectionTimeout
	if s.config.AWSIAMAuth != nil && s.config.AWSIAMAuth.ConnectionTimeout > 0 {
		timeout = s.config.AWSIAMAuth.ConnectionTimeout
	}

	testCtx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	if err := newDB.PingContext(testCtx); err != nil {
		s.logger.Error("Failed to ping database with refreshed token", "error", err, "endpoint", endpoint)
		newDB.Close()
		return
	}

	// Replace old connection with new one
	s.db = newDB

	// Close old connection in background to avoid blocking
	go func() {
		if err := oldDB.Close(); err != nil {
			s.logger.Warn("Failed to close old database connection", "error", err)
		}
	}()

	s.logger.Info("Successfully refreshed database connection with new IAM token", "endpoint", endpoint)
}
