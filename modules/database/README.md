# Database Module for Modular

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/database.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/database)
[![Modules CI](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml/badge.svg)](https://github.com/GoCodeAlone/modular/actions/workflows/modules-ci.yml)

A [Modular](https://github.com/GoCodeAlone/modular) module that provides database connectivity and management.

## Overview

The Database module provides a service for connecting to and interacting with SQL databases. It wraps the standard Go `database/sql` package to provide a clean, service-oriented interface that integrates with the Modular framework.

## Features

- Support for multiple database connections with named configurations
- Connection pooling with configurable settings
- Simplified interface for common database operations
- Context-aware database operations for proper cancellation and timeout handling
- Support for transactions

## Installation

```bash
go get github.com/GoCodeAlone/modular/modules/database
```

## Usage

### Importing Database Drivers

The database module uses the standard Go `database/sql` package, which requires you to import the specific database driver you plan to use as a side-effect. Make sure to import your desired driver in your main package:

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/database"
    
    // Import database drivers as needed
    _ "github.com/lib/pq"           // PostgreSQL driver
    _ "github.com/go-sql-driver/mysql"    // MySQL driver
    _ "github.com/mattn/go-sqlite3"  // SQLite driver
)
```

You'll also need to add the driver to your module's dependencies:

```bash
# For PostgreSQL
go get github.com/lib/pq

# For MySQL
go get github.com/go-sql-driver/mysql

# For SQLite
go get github.com/mattn/go-sqlite3
```

### Registering the Module

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/database"
    _ "github.com/lib/pq" // Import PostgreSQL driver
)

func main() {
    app := modular.NewStdApplication(
        modular.NewStdConfigProvider(configMap),
        logger,
    )
    
    // Register the database module
    app.RegisterModule(database.NewModule())
    
    // Register your modules that depend on the database service
    app.RegisterModule(NewYourModule())
    
    // Run the application
    if err := app.Run(); err != nil {
        logger.Error("Application error", "error", err)
    }
}
```

### Configuration

Configure the database module in your application configuration:

```yaml
database:
  default: "postgres_main"
  connections:
    postgres_main:
      driver: "postgres"
      dsn: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
      max_open_connections: 25
      max_idle_connections: 5
      connection_max_lifetime: 300  # seconds
      connection_max_idle_time: 60  # seconds
    mysql_reporting:
      driver: "mysql"
      dsn: "user:password@tcp(localhost:3306)/reporting"
      max_open_connections: 10
      max_idle_connections: 2
      connection_max_lifetime: 600  # seconds
```

### Using the Database Service

```go
type YourModule struct {
    dbService database.DatabaseService
}

// Request the database service
func (m *YourModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:     "database.service",
            Required: true,
        },
        // If you need a specific database connection:
        {
            Name:     "database.service.mysql_reporting",
            Required: true,
        },
    }
}

// Inject the service using constructor injection
func (m *YourModule) Constructor() modular.ModuleConstructor {
    return func(app *modular.StdApplication, services map[string]any) (modular.Module, error) {
        // Get the default database service
        dbService, ok := services["database.service"].(database.DatabaseService)
        if !ok {
            return nil, fmt.Errorf("service 'database.service' not found or wrong type")
        }
        
        // Get a specific database connection
        reportingDB, ok := services["database.service.mysql_reporting"].(database.DatabaseService)
        if !ok {
            return nil, fmt.Errorf("service 'database.service.mysql_reporting' not found or wrong type")
        }
        
        return &YourModule{
            dbService: dbService,
        }, nil
    }
}

// Example of using the database service
func (m *YourModule) GetUserData(ctx context.Context, userID int64) (*User, error) {
    user := &User{}
    
    row := m.dbService.QueryRowContext(ctx, 
        "SELECT id, name, email FROM users WHERE id = $1", userID)
    
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("user not found: %d", userID)
        }
        return nil, err
    }
    
    return user, nil
}

// Example of a transaction
func (m *YourModule) TransferFunds(ctx context.Context, fromAccount, toAccount int64, amount float64) error {
    tx, err := m.dbService.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    
    defer func() {
        if err != nil {
            tx.Rollback()
            return
        }
    }()
    
    // Debit from source account
    _, err = tx.ExecContext(ctx, 
        "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromAccount)
    if err != nil {
        return err
    }
    
    // Credit to destination account
    _, err = tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toAccount)
    if err != nil {
        return err
    }
    
    // Commit the transaction
    return tx.Commit()
}
```

### Working with multiple database connections

```go
type MultiDBModule struct {
    dbManager *database.Module
}

// Request the database manager
func (m *MultiDBModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:     "database.manager",
            Required: true,
        },
    }
}

// Inject the manager using constructor injection
func (m *MultiDBModule) Constructor() modular.ModuleConstructor {
    return func(app *modular.StdApplication, services map[string]any) (modular.Module, error) {
        dbManager, ok := services["database.manager"].(*database.Module)
        if !ok {
            return nil, fmt.Errorf("service 'database.manager' not found or wrong type")
        }
        
        return &MultiDBModule{
            dbManager: dbManager,
        }, nil
    }
}

// Example of using multiple database connections
func (m *MultiDBModule) ProcessData(ctx context.Context) error {
    // Get specific connections
    sourceDB, exists := m.dbManager.GetConnection("postgres_main")
    if !exists {
        return fmt.Errorf("source database connection not found")
    }
    
    reportingDB, exists := m.dbManager.GetConnection("mysql_reporting")
    if !exists {
        return fmt.Errorf("reporting database connection not found")
    }
    
    // Read from source
    rows, err := sourceDB.QueryContext(ctx, "SELECT id, data FROM source_table")
    if err != nil {
        return err
    }
    defer rows.Close()
    
    // Process and write to reporting
    for rows.Next() {
        var id int64
        var data string
        
        if err := rows.Scan(&id, &data); err != nil {
            return err
        }
        
        // Process and write to reporting DB
        _, err = reportingDB.ExecuteContext(ctx,
            "INSERT INTO processed_data (source_id, data) VALUES (?, ?)", 
            id, processData(data))
        if err != nil {
            return err
        }
    }
    
    return rows.Err()
}
```

## API Reference

### Types

#### DatabaseService

```go
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
```

#### Module Manager Methods

```go
// GetConnection returns a database service by name
func (m *Module) GetConnection(name string) (DatabaseService, bool)

// GetDefaultConnection returns the default database service
func (m *Module) GetDefaultConnection() DatabaseService

// GetConnections returns all configured database connections
func (m *Module) GetConnections() map[string]DatabaseService
```

## License

[MIT License](../../LICENSE)
