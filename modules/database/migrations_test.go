package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// mockEventEmitter is a no-op emitter used for tests
type mockEventEmitter struct{}

func (m *mockEventEmitter) EmitEvent(ctx context.Context, evt interface{}) error { return nil }

// openTestDB opens an in-memory SQLite database (CGO-free via modernc.org/sqlite)
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrationService_RunMigration_SuccessAndIdempotentRunner(t *testing.T) {
	db := openTestDB(t)
	svc := NewMigrationService(db, nil)
	runner := NewMigrationRunner(svc)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	migrations := []Migration{
		{ID: "001_create_table", Version: "001", SQL: "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)"},
		{ID: "002_add_index", Version: "002", SQL: "CREATE INDEX idx_test_name ON test(name)"},
	}

	if err := runner.RunMigrations(ctx, migrations); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	// Second run should be idempotent (no error, no duplicate application attempts)
	if err := runner.RunMigrations(ctx, migrations); err != nil {
		t.Fatalf("second run (idempotent) failed: %v", err)
	}

	// Validate applied migrations
	applied, err := svc.GetAppliedMigrations(ctx)
	if err != nil {
		t.Fatalf("failed to fetch applied migrations: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied migrations, got %d (%v)", len(applied), applied)
	}
}

func TestMigrationService_RunMigration_InvalidSQL(t *testing.T) {
	db := openTestDB(t)
	svc := NewMigrationService(db, nil)
	ctx := context.Background()
	// Ensure table exists
	if err := svc.CreateMigrationsTable(ctx); err != nil {
		t.Fatalf("failed to create migrations table: %v", err)
	}

	bad := Migration{ID: "bad_sql", Version: "003", SQL: "CREATE TABL broken"}
	err := svc.RunMigration(ctx, bad)
	if err == nil {
		t.Fatalf("expected error for invalid SQL, got nil")
	}
}

func TestMigrationService_TableNameValidation(t *testing.T) {
	db := openTestDB(t)
	svc := &migrationServiceImpl{db: db, tableName: "invalid-name!"}
	ctx := context.Background()
	if err := svc.CreateMigrationsTable(ctx); err == nil {
		t.Fatalf("expected validation error for invalid table name")
	}
}
