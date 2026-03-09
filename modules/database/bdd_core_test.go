package database

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
	_ "modernc.org/sqlite" // Import pure-Go SQLite driver for BDD tests (works with CGO_DISABLED)
)

// Database BDD Test Context
type DatabaseBDDTestContext struct {
	app             modular.Application
	module          *Module
	service         DatabaseService
	queryResult     interface{}
	queryError      error
	lastError       error
	transaction     *sql.Tx
	healthStatus    bool
	eventObserver   *TestEventObserver
	connectionError error
}

// TestEventObserver captures events for BDD testing
type TestEventObserver struct {
	mu     sync.RWMutex
	events []cloudevents.Event
	id     string
}

func newTestEventObserver() *TestEventObserver {
	return &TestEventObserver{
		id: "test-observer-database",
	}
}

func (o *TestEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	clone := event.Clone()
	o.mu.Lock()
	o.events = append(o.events, clone)
	o.mu.Unlock()
	return nil
}

func (o *TestEventObserver) ObserverID() string {
	return o.id
}

func (o *TestEventObserver) GetEvents() []cloudevents.Event {
	o.mu.RLock()
	defer o.mu.RUnlock()
	events := make([]cloudevents.Event, len(o.events))
	copy(events, o.events)
	return events
}

func (o *TestEventObserver) Reset() {
	o.mu.Lock()
	o.events = nil
	o.mu.Unlock()
}

func (ctx *DatabaseBDDTestContext) resetContext() {
	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.queryResult = nil
	ctx.queryError = nil
	ctx.lastError = nil
	ctx.transaction = nil
	ctx.healthStatus = false
	if ctx.eventObserver != nil {
		ctx.eventObserver.Reset()
	}
}

// Simple test logger for database BDD tests
type testLogger struct{}

func (l *testLogger) Debug(msg string, fields ...interface{}) {}
func (l *testLogger) Info(msg string, fields ...interface{})  {}
func (l *testLogger) Warn(msg string, fields ...interface{})  {}
func (l *testLogger) Error(msg string, fields ...interface{}) {}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestDatabaseModule runs the BDD tests for the database module
func TestDatabaseModule(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeDatabaseScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/database_module.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
