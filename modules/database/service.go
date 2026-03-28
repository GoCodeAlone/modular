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
	migrationService MigrationService
	eventEmitter     EventEmitter
	logger           modular.Logger // Logger service for error reporting
	ctx              context.Context
	cancel           context.CancelFunc
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
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	return service, nil
}

func (s *databaseServiceImpl) Connect() error {
	var db *sql.DB
	var err error

	// If AWS IAM authentication is enabled, use go-db-credential-refresh for automatic token management
	if s.config.AWSIAMAuth != nil && s.config.AWSIAMAuth.Enabled {
		s.logger.Info("Connecting to database with AWS IAM authentication using credential refresh")
		db, err = createDBWithCredentialRefresh(s.ctx, s.config, s.logger)
		if err != nil {
			s.logger.Error("AWS IAM authentication failed",
				"error", err.Error(),
				"troubleshooting_steps", []string{
					"1. Verify AWS credentials are available (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, or instance profile)",
					"2. Check IAM policy grants rds-db:connect for the database user",
					"3. Verify the database user exists and has rds_iam role (PostgreSQL: GRANT rds_iam TO user)",
					"4. Confirm the RDS endpoint and region are correct",
					"5. Ensure network connectivity to RDS (security groups, VPC, etc.)",
					"6. Check CloudTrail logs for IAM authentication attempts",
				})
			return fmt.Errorf("failed to create database connection with credential refresh: %w", err)
		}
	} else {
		// Standard connection without IAM authentication
		dsn := s.config.DSN

		// Preprocess DSN to handle special characters
		dsn, err = preprocessDSNForParsing(dsn)
		if err != nil {
			return fmt.Errorf("failed to preprocess DSN: %w", err)
		}

		db, err = sql.Open(s.config.Driver, dsn)
		if err != nil {
			return fmt.Errorf("failed to open database connection: %w", err)
		}

		// Configure connection pool
		configureConnectionPool(db, s.config)
	}

	// Test connection with configurable timeout
	timeout := DefaultConnectionTimeout
	if s.config.AWSIAMAuth != nil && s.config.AWSIAMAuth.ConnectionTimeout > 0 {
		timeout = s.config.AWSIAMAuth.ConnectionTimeout
	}
	s.logger.Debug("Testing database connection", "timeout", timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			s.logger.Error("Failed to ping database and close connection", "ping_error", err, "close_error", closeErr)
			return fmt.Errorf("failed to ping database and close connection: %w", err)
		}

		// Provide detailed diagnostics for ping failures
		if s.config.AWSIAMAuth != nil && s.config.AWSIAMAuth.Enabled {
			s.logger.Error("Database ping failed with IAM authentication",
				"error", err.Error(),
				"timeout", timeout,
				"possible_causes", []string{
					"IAM token generation failed",
					"Database user doesn't have rds_iam role",
					"IAM policy doesn't allow rds-db:connect",
					"Network connectivity issues",
					"Database is not accepting connections",
					"SSL/TLS configuration mismatch",
				})
		} else {
			s.logger.Error("Database ping failed",
				"error", err.Error(),
				"timeout", timeout)
		}
		return fmt.Errorf("failed to ping database: %w", err)
	}
	s.logger.Info("Database connection test successful")

	s.connMutex.Lock()
	s.db = db
	s.connMutex.Unlock()

	// Initialize migration service after successful connection
	if s.eventEmitter != nil {
		s.migrationService = NewMigrationService(s.db, s.eventEmitter)
	}

	return nil
}

func (s *databaseServiceImpl) Close() error {
	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Close database connection
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return fmt.Errorf("closing database connection: %w", err)
		}
		s.db = nil
	}
	return nil
}

func (s *databaseServiceImpl) DB() *sql.DB {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return s.db
}

func (s *databaseServiceImpl) Ping(ctx context.Context) error {
	s.connMutex.RLock()
	db := s.db
	s.connMutex.RUnlock()

	if db == nil {
		return ErrDatabaseNotConnected
	}
	err := db.PingContext(ctx)
	if err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	return nil
}

func (s *databaseServiceImpl) Stats() sql.DBStats {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
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
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in emit transaction committed event goroutine: %v", r)
				}
			}()
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
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in emit transaction rolled back event goroutine: %v", r)
				}
			}()
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
