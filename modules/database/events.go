package database

// Event type constants for database module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Connection events
	EventTypeConnected       = "com.modular.database.connected"
	EventTypeDisconnected    = "com.modular.database.disconnected"
	EventTypeConnectionError = "com.modular.database.connection.error"

	// Query events
	EventTypeQueryExecuted = "com.modular.database.query.executed"
	EventTypeQueryError    = "com.modular.database.query.error"

	// Transaction events
	EventTypeTransactionStarted    = "com.modular.database.transaction.started"
	EventTypeTransactionCommitted  = "com.modular.database.transaction.committed"
	EventTypeTransactionRolledBack = "com.modular.database.transaction.rolledback"

	// Migration events
	EventTypeMigrationStarted   = "com.modular.database.migration.started"
	EventTypeMigrationCompleted = "com.modular.database.migration.completed"
	EventTypeMigrationFailed    = "com.modular.database.migration.failed"

	// Configuration events
	EventTypeConfigLoaded = "com.modular.database.config.loaded"
)
