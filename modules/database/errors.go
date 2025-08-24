package database

import "errors"

// Static error definitions to avoid dynamic error creation (err113 linter)
var (
	// ErrTransactionNil is returned when a nil transaction is passed to transaction operations
	ErrTransactionNil = errors.New("transaction cannot be nil")

	// ErrInvalidTableName is returned when an invalid table name is used
	ErrInvalidTableName = errors.New("invalid table name: must start with letter/underscore and contain only alphanumeric/underscore characters")

	// ErrMigrationServiceNotInitialized is returned when migration operations are attempted
	// without proper migration service initialization
	ErrMigrationServiceNotInitialized = errors.New("migration service not initialized")
)
