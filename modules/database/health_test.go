package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // SQLite driver for tests
)

func TestModule_HealthCheck_WithHealthyDatabase(t *testing.T) {
	// RED PHASE: Write failing test first

	// Create a module with a healthy database connection
	module := &Module{
		config: &Config{
			Default: "test",
			Connections: map[string]*ConnectionConfig{
				"test": {
					Driver: "sqlite",
					DSN:    ":memory:",
				},
			},
		},
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}

	// Initialize the module to establish connections
	err := module.initializeConnections()
	require.NoError(t, err)

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return healthy status
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	// Find the database connection report
	var dbReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "database" {
			dbReport = &reports[i]
			break
		}
	}

	require.NotNil(t, dbReport, "Expected database health report")
	assert.Equal(t, "database", dbReport.Module)
	assert.Equal(t, modular.HealthStatusHealthy, dbReport.Status)
	assert.NotEmpty(t, dbReport.Message)
	assert.False(t, dbReport.Optional)
	assert.WithinDuration(t, time.Now(), dbReport.CheckedAt, 5*time.Second)
}

func TestModule_HealthCheck_WithUnhealthyDatabase(t *testing.T) {
	// RED PHASE: Test unhealthy database scenario

	// Create a module with no connections (simulating unhealthy state)
	module := &Module{
		config: &Config{
			Default:     "test",
			Connections: map[string]*ConnectionConfig{},
		},
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return unhealthy status
	assert.NoError(t, err)
	assert.NotEmpty(t, reports)

	// Find the database connection report
	var dbReport *modular.HealthReport
	for i, report := range reports {
		if report.Module == "database" {
			dbReport = &reports[i]
			break
		}
	}

	require.NotNil(t, dbReport, "Expected database health report")
	assert.Equal(t, "database", dbReport.Module)
	assert.Equal(t, modular.HealthStatusUnhealthy, dbReport.Status)
	assert.Contains(t, dbReport.Message, "no connections available")
	assert.False(t, dbReport.Optional)
	assert.WithinDuration(t, time.Now(), dbReport.CheckedAt, 5*time.Second)
}

func TestModule_HealthCheck_MultipleConnections(t *testing.T) {
	// RED PHASE: Test multiple database connections

	// Create a module with multiple connections
	module := &Module{
		config: &Config{
			Default: "primary",
			Connections: map[string]*ConnectionConfig{
				"primary": {
					Driver: "sqlite",
					DSN:    ":memory:",
				},
				"secondary": {
					Driver: "sqlite",
					DSN:    ":memory:",
				},
			},
		},
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}

	// Initialize the module to establish connections
	err := module.initializeConnections()
	require.NoError(t, err)

	// Act: Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reports, err := module.HealthCheck(ctx)

	// Assert: Should return separate reports for each connection
	assert.NoError(t, err)
	assert.Len(t, reports, 2)

	// Verify each connection has a health report
	connectionNames := make(map[string]bool)
	for _, report := range reports {
		assert.Equal(t, "database", report.Module)
		assert.Equal(t, modular.HealthStatusHealthy, report.Status)
		assert.False(t, report.Optional)
		connectionNames[report.Component] = true
	}

	assert.True(t, connectionNames["primary"])
	assert.True(t, connectionNames["secondary"])
}

func TestModule_HealthCheck_WithContext(t *testing.T) {
	// RED PHASE: Test context cancellation handling

	// Create a module with connections
	module := &Module{
		config: &Config{
			Default: "test",
			Connections: map[string]*ConnectionConfig{
				"test": {
					Driver: "sqlite",
					DSN:    ":memory:",
				},
			},
		},
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}

	// Initialize the module to establish connections
	err := module.initializeConnections()
	require.NoError(t, err)

	// Act: Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reports, err := module.HealthCheck(ctx)

	// Assert: Should handle context cancellation gracefully
	// The exact behavior depends on implementation but should not panic
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	} else {
		// If no error, reports should still be valid
		assert.NotNil(t, reports)
	}
}

// Test helper to verify the BDD test expectations are met
func TestModule_ImplementsHealthProvider(t *testing.T) {
	// Verify that Module implements HealthProvider interface
	module := &Module{
		connections: make(map[string]*sql.DB),
		services:    make(map[string]DatabaseService),
	}

	// This should compile without errors if the interface is properly implemented
	var _ modular.HealthProvider = module

	// Also verify method signatures exist (will fail to compile if missing)
	ctx := context.Background()
	reports, err := module.HealthCheck(ctx)

	// No error expected with an initialized module, even if empty
	assert.NoError(t, err)
	assert.NotNil(t, reports)
	// Should report unhealthy because no connections
	assert.Len(t, reports, 1)
	assert.Equal(t, modular.HealthStatusUnhealthy, reports[0].Status)
}
