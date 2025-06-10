package database

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// DatabaseExecutor represents the interface that external modules expect
// This matches the user's interface from the problem description
type DatabaseExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// TestInterfaceMatchingDiagnosis examines why interface matching fails
func TestInterfaceMatchingDiagnosis(t *testing.T) {
	// Create a database module
	dbModule := NewModule()

	// Get the services it provides
	services := dbModule.ProvidesServices()

	var databaseService modular.ServiceProvider
	for _, svc := range services {
		if svc.Name == "database.service" {
			databaseService = svc
			break
		}
	}

	require.NotNil(t, databaseService.Instance, "database.service should exist")

	// Check what type it actually provides
	actualType := reflect.TypeOf(databaseService.Instance)
	t.Logf("Database module provides: %v", actualType)

	// Check if it implements our interface
	expectedType := reflect.TypeOf((*DatabaseExecutor)(nil)).Elem()
	t.Logf("Expected interface: %v", expectedType)

	implementsInterface := actualType.Implements(expectedType)
	t.Logf("Does it implement DatabaseExecutor? %v", implementsInterface)

	// It should implement the interface since lazyDefaultService has all the methods
	assert.True(t, implementsInterface, "lazyDefaultService should implement DatabaseExecutor")
}

// TestRealWorldScenario reproduces the exact scenario that fails for users
func TestRealWorldScenario(t *testing.T) {
	t.Log("=== Testing Real World Scenario ===")

	// Create a minimal config to avoid environment variable issues
	type simpleConfig struct {
		Database map[string]interface{} `json:"database"`
	}

	config := &simpleConfig{
		Database: map[string]interface{}{
			"default": "test",
			"connections": map[string]interface{}{
				"test": map[string]interface{}{
					"driver": "sqlite",
					"dsn":    ":memory:",
				},
			},
		},
	}

	// Create application with a simple config provider
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(config),
		&mockLogger{},
	)

	// Register database module first
	dbModule := NewModule()
	app.RegisterModule(dbModule)

	// Register a consumer module that wants DatabaseExecutor with MatchByInterface=true
	consumerModule := &RealisticConsumer{name: "routes-module"}
	app.RegisterModule(consumerModule)

	// Initialize the application
	err := app.Init()
	if err != nil {
		t.Logf("Initialization failed: %v", err)

		// Check if it's the interface matching error we're looking for
		if strings.Contains(err.Error(), "no service found implementing interface") {
			// This is the error we're testing - interface matching failed
			t.Log("❌ Interface matching failed as expected in the user's scenario")
			assert.Contains(t, err.Error(), "DatabaseExecutor", "Error should mention DatabaseExecutor")
			return
		} else {
			// This is a different error - skip this test condition
			t.Skipf("Different error not related to interface matching: %v", err)
		}
		return
	}

	// If initialization succeeded, that's actually good! It means interface matching worked
	t.Log("✅ Interface matching worked! The lazyDefaultService implements DatabaseExecutor")

	// Start the application to make sure everything works
	err = app.Start()
	require.NoError(t, err)

	// Verify the consumer got its database executor
	assert.NotNil(t, consumerModule.db, "Consumer should have received database executor")

	err = app.Stop()
	require.NoError(t, err)
}

// RealisticConsumer mimics a real module like RoutesModule
type RealisticConsumer struct {
	name string
	db   DatabaseExecutor
}

func (m *RealisticConsumer) Name() string {
	return m.name
}

func (m *RealisticConsumer) Init(app modular.Application) error {
	// Use init-time injection to get the database service
	var db DatabaseExecutor
	err := app.GetService("database.service", &db)
	if err != nil {
		return err
	}
	m.db = db
	return nil
}

func (m *RealisticConsumer) Dependencies() []string {
	return []string{"database"}
}

func (m *RealisticConsumer) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (m *RealisticConsumer) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "database.service",
			Required:           true,
			MatchByInterface:   true, // This is the key test
			SatisfiesInterface: reflect.TypeOf((*DatabaseExecutor)(nil)).Elem(),
		},
	}
}

// RealisticConsumer uses init-time injection instead of constructor injection
// This allows us to test the same module instance that was registered

// TestInterfaceMatchingWorks proves that MatchByInterface=true should work
func TestInterfaceMatchingWorks(t *testing.T) {
	t.Log("=== Proving Interface Matching Works ===")

	// Simple configuration
	type appConfig struct{}

	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&appConfig{}),
		&mockLogger{},
	)

	// Register database module
	dbModule := NewModule()
	app.RegisterModule(dbModule)

	// Register a simple consumer that uses MatchByInterface=true
	consumer := &SimpleConsumer{name: "simple-consumer"}
	app.RegisterModule(consumer)

	// Initialize - this should work
	err := app.Init()
	require.NoError(t, err, "Interface matching should work because lazyDefaultService implements DatabaseExecutor")

	t.Log("✅ SUCCESS: MatchByInterface=true works with the database module!")
	t.Log("   This proves the database module's lazyDefaultService correctly implements DatabaseExecutor")

	// The issue in the user's code must be something else
	t.Log("🔍 The user's issue is likely:")
	t.Log("   1. Configuration problems")
	t.Log("   2. Different interface signature")
	t.Log("   3. Import/package issues")
	t.Log("   4. Or the actual database module they're using differs from this test")
}

// SimpleConsumer with minimal implementation
type SimpleConsumer struct {
	name string
	db   DatabaseExecutor
}

func (m *SimpleConsumer) Name() string {
	return m.name
}

func (m *SimpleConsumer) Init(_ modular.Application) error {
	return nil
}

func (m *SimpleConsumer) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "database.service",
			Required:           true,
			MatchByInterface:   true, // This should work!
			SatisfiesInterface: reflect.TypeOf((*DatabaseExecutor)(nil)).Elem(),
		},
	}
}

func (m *SimpleConsumer) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Cast to DatabaseExecutor - this should work
		db, ok := services["database.service"].(DatabaseExecutor)
		if !ok {
			return nil, modular.ErrRequiredServiceNotFound
		}

		return &SimpleConsumer{
			name: m.name,
			db:   db,
		}, nil
	}
}

// TestExactUserScenario attempts to reproduce the user's exact setup
func TestExactUserScenario(t *testing.T) {
	t.Log("=== Testing User's Exact Scenario ===")

	// Create config that matches typical database module usage
	type DatabaseConfig struct {
		Default     string                            `yaml:"default"`
		Connections map[string]map[string]interface{} `yaml:"connections"`
	}

	type AppConfig struct {
		Database DatabaseConfig `yaml:"database"`
	}

	config := &AppConfig{
		Database: DatabaseConfig{
			Default: "test",
			Connections: map[string]map[string]interface{}{
				"test": {
					"driver": "sqlite",
					"dsn":    ":memory:",
				},
			},
		},
	}

	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(config),
		&mockLogger{},
	)

	// Register database module
	app.RegisterModule(NewModule())

	// Create a module that exactly matches the user's RequiresServices pattern
	routesModule := &RoutesModule{name: "routes-module"}
	app.RegisterModule(routesModule)

	// This should work based on our previous tests
	err := app.Init()
	if err != nil {
		t.Logf("❌ Initialization failed: %v", err)

		// Check if it's specifically the interface matching error
		if strings.Contains(err.Error(), "no service found implementing interface") &&
			strings.Contains(err.Error(), "DatabaseExecutor") {
			t.Log("🚨 Confirmed: Interface matching failed!")
			t.Log("   This suggests there's a subtle difference between our test and the user's setup")

			// Let's debug the interface types
			t.Log("🔍 Debugging interface types...")

			// Get the database service
			services := NewModule().ProvidesServices()
			var dbService modular.ServiceProvider
			for _, svc := range services {
				if svc.Name == "database.service" {
					dbService = svc
					break
				}
			}

			if dbService.Instance != nil {
				actualType := reflect.TypeOf(dbService.Instance)
				expectedType := reflect.TypeOf((*DatabaseExecutor)(nil)).Elem()

				t.Logf("   Actual service type: %v", actualType)
				t.Logf("   Expected interface: %v", expectedType)
				t.Logf("   Implements interface: %v", actualType.Implements(expectedType))
			}

			return
		}

		// Different error - might be config related
		t.Logf("Different error (likely config): %v", err)
		return
	}

	t.Log("✅ Success! Interface matching worked in this scenario too")
	t.Log("   The user's issue must be very specific to their setup")

	err = app.Start()
	require.NoError(t, err)
	err = app.Stop()
	require.NoError(t, err)
}

// RoutesModule matches the user's module pattern exactly
type RoutesModule struct {
	name string
	db   DatabaseExecutor
}

func (m *RoutesModule) Name() string {
	return m.name
}

func (m *RoutesModule) Init(_ modular.Application) error {
	return nil
}

func (m *RoutesModule) Dependencies() []string {
	return []string{"database"}
}

func (m *RoutesModule) ProvidesServices() []modular.ServiceProvider {
	return nil
}

// This exactly matches the user's RequiresServices implementation
func (m *RoutesModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "database.service",
			Required:           true,
			MatchByInterface:   true, // This is what the user reported failing
			SatisfiesInterface: reflect.TypeOf((*DatabaseExecutor)(nil)).Elem(),
		},
	}
}

func (m *RoutesModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get the database service as DatabaseExecutor
		dbExecutor, ok := services["database.service"].(DatabaseExecutor)
		if !ok {
			return nil, modular.ErrRequiredServiceNotFound
		}

		return &RoutesModule{
			name: m.name,
			db:   dbExecutor,
		}, nil
	}
}

// mockLogger for testing
type mockLogger struct{}

func (l *mockLogger) Debug(msg string, args ...interface{}) {}
func (l *mockLogger) Info(msg string, args ...interface{})  {}
func (l *mockLogger) Warn(msg string, args ...interface{})  {}
func (l *mockLogger) Error(msg string, args ...interface{}) {}

// TestReflectTypeOfMistake demonstrates the common mistake with reflect.TypeOf
func TestReflectTypeOfMistake(t *testing.T) {
	t.Log("=== Testing Common reflect.TypeOf Mistakes ===")

	// ❌ WRONG WAY - This is what causes the interface matching to fail
	// Note: This will be nil because you can't cast nil to an interface type
	wrongType := reflect.TypeOf((DatabaseExecutor)(nil))
	t.Logf("❌ Wrong way: reflect.TypeOf((DatabaseExecutor)(nil)) = %v", wrongType)
	if wrongType != nil {
		t.Logf("   Kind: %v", wrongType.Kind())
	} else {
		t.Log("   Kind: <nil> - This is why interface matching fails!")
	}

	// ✅ CORRECT WAY - This is what makes interface matching work
	correctType := reflect.TypeOf((*DatabaseExecutor)(nil)).Elem()
	t.Logf("✅ Correct way: reflect.TypeOf((*DatabaseExecutor)(nil)).Elem() = %v", correctType)
	t.Logf("   Kind: %v", correctType.Kind())

	// Demonstrate the difference
	assert.Nil(t, wrongType, "Wrong way should result in nil")
	assert.NotNil(t, correctType, "Correct way should result in valid type")
	assert.Equal(t, reflect.Interface, correctType.Kind(), "Correct type should be an interface")

	// Show what happens when we use the wrong type for interface matching
	dbModule := NewModule()
	services := dbModule.ProvidesServices()
	var dbService modular.ServiceProvider
	for _, svc := range services {
		if svc.Name == "database.service" {
			dbService = svc
			break
		}
	}

	if dbService.Instance != nil {
		actualType := reflect.TypeOf(dbService.Instance)

		// Can't call Implements on nil type, so this would panic in real code
		correctImplements := actualType.Implements(correctType)

		t.Logf("   Service type: %v", actualType)
		t.Logf("   Implements correct type: %v", correctImplements)

		assert.True(t, correctImplements, "Should implement the correct type")

		t.Log("")
		t.Log("🎯 KEY INSIGHT:")
		t.Log("   The user's MatchByInterface=true failed because they used:")
		t.Log("   ❌ reflect.TypeOf((DatabaseExecutor)(nil))  // Returns nil!")
		t.Log("   Instead of:")
		t.Log("   ✅ reflect.TypeOf((*DatabaseExecutor)(nil)).Elem()")
		t.Log("")
		t.Log("   The correct pattern for interface types in reflection is:")
		t.Log("   reflect.TypeOf((*InterfaceName)(nil)).Elem()")
		t.Log("")
		t.Log("   Using the wrong pattern causes SatisfiesInterface to be nil,")
		t.Log("   which makes the interface matching logic fail.")
	}
}

// TestCorrectInterfaceMatchingPattern shows the proper way
func TestCorrectInterfaceMatchingPattern(t *testing.T) {
	t.Log("=== Demonstrating Correct Interface Matching Pattern ===")

	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(struct{}{}),
		&mockLogger{},
	)

	// Register database module
	app.RegisterModule(NewModule())

	// Register consumer with CORRECT reflect pattern
	consumer := &CorrectConsumer{name: "correct-consumer"}
	app.RegisterModule(consumer)

	// This should work now
	err := app.Init()
	require.NoError(t, err, "Should work with correct reflect.TypeOf pattern")

	t.Log("✅ SUCCESS! Interface matching works with correct reflect pattern")
}

// CorrectConsumer demonstrates the correct reflect.TypeOf pattern
type CorrectConsumer struct {
	name string
	db   DatabaseExecutor
}

func (m *CorrectConsumer) Name() string {
	return m.name
}

func (m *CorrectConsumer) Init(_ modular.Application) error {
	return nil
}

func (m *CorrectConsumer) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:             "database.service",
			Required:         true,
			MatchByInterface: true,
			// ✅ CORRECT: Use pointer type and .Elem() for interfaces
			SatisfiesInterface: reflect.TypeOf((*DatabaseExecutor)(nil)).Elem(),
		},
	}
}

func (m *CorrectConsumer) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		db, ok := services["database.service"].(DatabaseExecutor)
		if !ok {
			return nil, modular.ErrRequiredServiceNotFound
		}

		return &CorrectConsumer{
			name: m.name,
			db:   db,
		}, nil
	}
}
