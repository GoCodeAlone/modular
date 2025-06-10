package modular

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// DatabaseExecutor matches the user's interface from the problem description
type DatabaseExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Ensure that *sql.DB implements our interface
var _ DatabaseExecutor = (*sql.DB)(nil)

// mockDatabaseExecutor is a mock implementation for testing
type mockDatabaseExecutor struct{}

func (m *mockDatabaseExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return &mockResult{}, nil
}

func (m *mockDatabaseExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (m *mockDatabaseExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return &sql.Row{}
}

func (m *mockDatabaseExecutor) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, nil
}

func (m *mockDatabaseExecutor) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, nil
}

// mockResult implements sql.Result for testing
type mockResult struct{}

func (m *mockResult) LastInsertId() (int64, error) { return 0, nil }
func (m *mockResult) RowsAffected() (int64, error) { return 0, nil }

// Ensure our mock implements the interface
var _ DatabaseExecutor = (*mockDatabaseExecutor)(nil)

// MockDatabaseService simulates what the database module provides
type MockDatabaseService interface {
	DB() DatabaseExecutor
	Close() error
}

// mockDatabaseServiceImpl simulates the lazy wrapper that the database module uses
type mockDatabaseServiceImpl struct {
	executor DatabaseExecutor
}

func (m *mockDatabaseServiceImpl) DB() DatabaseExecutor {
	return m.executor
}

func (m *mockDatabaseServiceImpl) Close() error {
	return nil
}

// TestInterfaceMatchingCore demonstrates the core issue with interface matching
func TestInterfaceMatchingCore(t *testing.T) {
	// Create a mock database service (simulating what the database module provides)
	mockExecutor := &mockDatabaseExecutor{}
	mockService := &mockDatabaseServiceImpl{executor: mockExecutor}

	// Test 1: Check if mockDatabaseExecutor implements DatabaseExecutor (it should)
	expectedType := reflect.TypeOf((*DatabaseExecutor)(nil)).Elem()
	mockExecutorType := reflect.TypeOf((*mockDatabaseExecutor)(nil))

	assert.True(t, mockExecutorType.Implements(expectedType),
		"mockDatabaseExecutor should implement DatabaseExecutor interface")

	// Test 2: Check if mockDatabaseServiceImpl implements DatabaseExecutor (it should NOT)
	mockServiceType := reflect.TypeOf((*mockDatabaseServiceImpl)(nil))
	assert.False(t, mockServiceType.Implements(expectedType),
		"mockDatabaseServiceImpl should NOT implement DatabaseExecutor interface")

	// Test 3: Check if mockDatabaseServiceImpl implements MockDatabaseService (it should)
	mockDBServiceType := reflect.TypeOf((*MockDatabaseService)(nil)).Elem()
	assert.True(t, mockServiceType.Implements(mockDBServiceType),
		"mockDatabaseServiceImpl should implement MockDatabaseService interface")

	// Test 4: Demonstrate the workaround - extract the DatabaseExecutor from the service
	actualDB := mockService.DB()
	actualDBType := reflect.TypeOf(actualDB)
	assert.True(t, actualDBType.Implements(expectedType),
		"The DatabaseExecutor returned by the service should implement DatabaseExecutor")

	t.Log("✅ Core issue demonstrated:")
	t.Log("   - mockDatabaseExecutor implements DatabaseExecutor ✓")
	t.Log("   - Database service wrapper does NOT implement DatabaseExecutor ✗")
	t.Log("   - But wrapper.DB() returns DatabaseExecutor which does implement DatabaseExecutor ✓")
}

// TestServiceDependencyMatching simulates the service dependency resolution
func TestServiceDependencyMatching(t *testing.T) {
	// Create a test config that won't fail environment parsing
	config := struct {
		TestKey string `yaml:"test_key" default:"test_value"`
	}{}

	// Create a test application with proper config
	app := NewStdApplication(
		NewStdConfigProvider(config),
		&logger{t},
	)

	// Create a mock database module that provides a wrapper service
	mockDBModule := &MockDatabaseModule{name: "mock-database"}
	app.RegisterModule(mockDBModule)

	// Create a module that tries to use MatchByInterface=true with WRONG reflection syntax
	failingModule := &FailingTestModule{name: "failing-module"}
	app.RegisterModule(failingModule)

	// This should fail with MatchByInterface=true due to the incorrect reflect.TypeOf pattern
	err := app.Init()
	assert.Error(t, err, "Should fail when using incorrect reflect.TypeOf pattern")
	if err != nil {
		t.Logf("Expected failure: %v", err)
		// The error should be about invalid interface configuration due to nil SatisfiesInterface
		assert.Contains(t, err.Error(), "invalid interface configuration", "Error should mention invalid interface configuration")
		assert.Contains(t, err.Error(), "SatisfiesInterface is nil", "Error should mention SatisfiesInterface is nil")
		assert.Contains(t, err.Error(), "hint: use reflect.TypeOf((*InterfaceName)(nil)).Elem()", "Error should provide usage hint")
	}
}

// MockDatabaseModule simulates the database module
type MockDatabaseModule struct {
	name string
}

func (m *MockDatabaseModule) Name() string {
	return m.name
}

func (m *MockDatabaseModule) Init(_ Application) error {
	return nil
}

func (m *MockDatabaseModule) Dependencies() []string {
	return nil
}

func (m *MockDatabaseModule) ProvidesServices() []ServiceProvider {
	// Create a dummy mock executor for testing
	mockExecutor := &mockDatabaseExecutor{}
	wrapper := &mockDatabaseServiceImpl{executor: mockExecutor}

	return []ServiceProvider{
		{
			Name:        "database.service",
			Description: "Database service wrapper",
			Instance:    wrapper, // Provide wrapper, not direct executor
		},
	}
}

func (m *MockDatabaseModule) RequiresServices() []ServiceDependency {
	return nil
}

// FailingTestModule tries to use MatchByInterface=true (will fail)
type FailingTestModule struct {
	name string
}

func (m *FailingTestModule) Name() string {
	return m.name
}

func (m *FailingTestModule) Init(_ Application) error {
	return nil
}

func (m *FailingTestModule) Dependencies() []string {
	return []string{"mock-database"}
}

func (m *FailingTestModule) ProvidesServices() []ServiceProvider {
	return nil
}

func (m *FailingTestModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{
		{
			Name:               "nonexistent.service", // Change to a service name that doesn't exist
			Required:           true,
			MatchByInterface:   true,                                    // This will fail due to incorrect reflect pattern
			SatisfiesInterface: reflect.TypeOf((DatabaseExecutor)(nil)), // ❌ WRONG - returns nil
		},
	}
}

func (m *FailingTestModule) Constructor() ModuleConstructor {
	return func(app Application, services map[string]any) (Module, error) {
		// This won't be called because dependency resolution will fail
		return m, nil
	}
}
