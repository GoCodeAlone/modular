//go:build !cgo

package database_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"testing"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/database"

	_ "modernc.org/sqlite" // Import pure Go SQLite driver
)

// Define static errors
var (
	ErrInvalidDatabaseService = fmt.Errorf("service is not of type database.DatabaseService or is nil")
)

// Package database_test provides tests for the database module
// use sqlite3 in memory for testing

// Example showing how to use DatabaseService in another module
func TestExample_dependentModule(t *testing.T) {
	dbConfigJSON := `
database:
  connections:
    default:
      driver: sqlite
      dsn: ":memory:"
  default: default
`
	// write the config to a file
	configFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	defer func() {
		if err := os.Remove(configFile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	if _, err = configFile.WriteString(dbConfigJSON); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	if err = configFile.Close(); err != nil {
		t.Fatalf("Failed to close config file: %v", err)
	}

	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(nil),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelDebug,
			},
		)),
	)

	// Inject feeders for this app instance only (avoid mutating global state)
	if stdApp, ok := app.(*modular.StdApplication); ok {
		stdApp.SetConfigFeeders([]modular.Feeder{feeders.NewYamlFeeder(configFile.Name())})
	} else {
		t.Fatalf("unexpected application concrete type: %T", app)
	}

	app.RegisterModule(database.NewModule())
	app.RegisterModule(&YourModule{t: t})

	if err = app.Init(); err != nil {
		t.Errorf("Failed to initialize application: %v", err)
	}
	if err = app.Start(); err != nil {
		t.Errorf("Failed to start application: %v", err)
	}
	if err = app.Stop(); err != nil {
		t.Errorf("Failed to stop application: %v", err)
	}
}

type YourModule struct {
	t         *testing.T
	app       modular.Application
	dbService database.DatabaseService
	shutdown  []func()
}

func (y *YourModule) RegisterConfig(modular.Application) {
}

func (y *YourModule) Init(_ modular.Application) error {
	_, err := y.dbService.Execute("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		return fmt.Errorf("failed to create test table: %w", err)
	}
	return nil
}

func (y *YourModule) Start(_ context.Context) error {
	_, err := y.dbService.Execute("INSERT INTO test (name) VALUES ('test')")
	if err != nil {
		return fmt.Errorf("failed to insert test data: %w", err)
	}

	rows, err := y.dbService.Query("SELECT id, name FROM test")
	if err != nil {
		return fmt.Errorf("failed to query test data: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			y.t.Logf("Failed to close rows: %v", err)
		}
	}()
	for rows.Next() {
		var id int
		var name string
		if scanErr := rows.Scan(&id, &name); scanErr != nil {
			return fmt.Errorf("failed to scan test data: %w", scanErr)
		}
		y.t.Logf("Test data: id=%d, name=%s", id, name)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating over test data: %w", err)
	}

	return nil
}

func (y *YourModule) Stop(_ context.Context) error {
	return nil
}

func (y *YourModule) Name() string {
	return "yourmodule"
}

func (y *YourModule) Dependencies() []string {
	return []string{database.Name}
}

func (y *YourModule) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (y *YourModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "database.service",
			Required:           true,
			SatisfiesInterface: reflect.TypeOf((*database.DatabaseService)(nil)).Elem(),
		},
	}
}

func (y *YourModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get the JSONSchemaService from the services map
		dbService, ok := services["database.service"].(database.DatabaseService)
		if !ok {
			return nil, fmt.Errorf("%w: detected type %T", ErrInvalidDatabaseService, services["database.service"])
		}

		return &YourModule{
			t:         y.t,
			app:       app,
			dbService: dbService,
			shutdown:  []func(){},
		}, nil
	}
}
