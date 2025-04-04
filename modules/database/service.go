package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Define static errors
var (
	ErrEmptyDriver          = errors.New("database driver cannot be empty")
	ErrEmptyDSN             = errors.New("database connection string (DSN) cannot be empty")
	ErrDatabaseNotConnected = errors.New("database not connected")
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

	// ExecuteContext executes a query without returning any rows
	ExecuteContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Execute executes a query without returning any rows (using default context)
	Execute(query string, args ...interface{}) (sql.Result, error)

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
}

// databaseServiceImpl implements the DatabaseService interface
type databaseServiceImpl struct {
	config ConnectionConfig
	db     *sql.DB
}

// NewDatabaseService creates a new database service from configuration
func NewDatabaseService(config ConnectionConfig) (DatabaseService, error) {
	if config.Driver == "" {
		return nil, ErrEmptyDriver
	}

	if config.DSN == "" {
		return nil, ErrEmptyDSN
	}

	return &databaseServiceImpl{
		config: config,
	}, nil
}

func (s *databaseServiceImpl) Connect() error {
	db, err := sql.Open(s.config.Driver, s.config.DSN)
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
		db.SetConnMaxLifetime(time.Duration(s.config.ConnectionMaxLifetime) * time.Second)
	}
	if s.config.ConnectionMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(s.config.ConnectionMaxIdleTime) * time.Second)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.db = db
	return nil
}

func (s *databaseServiceImpl) Close() error {
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return fmt.Errorf("closing database connection: %w", err)
		}
	}
	return nil
}

func (s *databaseServiceImpl) DB() *sql.DB {
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

func (s *databaseServiceImpl) ExecuteContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if s.db == nil {
		return nil, ErrDatabaseNotConnected
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	return result, nil
}

func (s *databaseServiceImpl) Execute(query string, args ...interface{}) (sql.Result, error) {
	return s.ExecuteContext(context.Background(), query, args...)
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
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning database transaction: %w", err)
	}
	return tx, nil
}
