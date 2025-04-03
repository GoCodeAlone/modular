package database_test

import (
	"context"
	"fmt"
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/database"
	"log/slog"
	"os"
	"reflect"
	"testing"

	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
)

// Package database_test provides tests for the database module
// use sqlite3 in memory for testing

// Example showing how to use JSONSchemaService in another module
func TestExample_dependentModule(t *testing.T) {
	dbConfigJson := `
database:
  connections:
    default:
      driver: sqlite3
      dsn: ":memory:"
  default: default
`
	// write the config to a file
	configFile, err := os.CreateTemp("", "config.yaml")
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	defer os.Remove(configFile.Name())

	if _, err = configFile.WriteString(dbConfigJson); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	configFile.Close()

	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder(configFile.Name()),
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

func (y *YourModule) Init(app modular.Application) error {
	_, err := y.dbService.Execute("CREATE TABLE IF NOT EXISTS test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		return fmt.Errorf("failed to create test table: %w", err)
	}
	return nil
}

func (y *YourModule) Start(ctx context.Context) error {
	_, err := y.dbService.Execute("INSERT INTO test (name) VALUES ('test')")
	if err != nil {
		return fmt.Errorf("failed to insert test data: %w", err)
	}

	rows, err := y.dbService.Query("SELECT id, name FROM test")
	if err != nil {
		return fmt.Errorf("failed to query test data: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return fmt.Errorf("failed to scan test data: %w", err)
		}
		y.t.Logf("Test data: id=%d, name=%s", id, name)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating over test data: %w", err)
	}

	return nil
}

func (y *YourModule) Stop(ctx context.Context) error {
	return nil
}

func (y *YourModule) Name() string {
	return "yourmodule"
}

func (y *YourModule) Dependencies() []string {
	return []string{"database"}
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
	return func(app *modular.StdApplication, services map[string]any) (modular.Module, error) {
		// Get the JSONSchemaService from the services map
		dbService, ok := services["database.service"].(database.DatabaseService)
		if !ok {
			return nil, fmt.Errorf("service 'database.service' is not of type database.DatabaseService or is nil. Detected type: %T", services["database.service"])
		}

		return &YourModule{
			t:         y.t,
			app:       app,
			dbService: dbService,
			shutdown:  []func(){},
		}, nil
	}
}
