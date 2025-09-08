package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// HealthCheck implements the HealthProvider interface for the database module.
// This method checks the health of all configured database connections and
// returns detailed reports for each connection.
//
// The health check performs the following for each connection:
//   - Tests connectivity using database Ping
//   - Reports connection pool statistics
//   - Provides detailed error information if connections fail
//
// Returns:
//   - Slice of HealthReport objects, one for each database connection
//   - Error if the health check operation itself fails
func (m *Module) HealthCheck(ctx context.Context) ([]modular.HealthReport, error) {
	reports := make([]modular.HealthReport, 0)
	checkTime := time.Now()

	// If no connections are configured, report unhealthy
	if len(m.connections) == 0 {
		report := modular.HealthReport{
			Module:        "database",
			Component:     "connections",
			Status:        modular.HealthStatusUnhealthy,
			Message:       "no connections available",
			CheckedAt:     checkTime,
			ObservedSince: checkTime,
			Optional:      false,
			Details: map[string]any{
				"configured_connections": 0,
				"active_connections":     0,
			},
		}
		reports = append(reports, report)
		return reports, nil
	}

	// Check health of each configured connection
	for name, db := range m.connections {
		report := m.checkConnectionHealth(ctx, name, db, checkTime)
		reports = append(reports, report)
	}

	return reports, nil
}

// checkConnectionHealth performs a health check on a single database connection
// and returns a detailed health report with connection statistics and status.
func (m *Module) checkConnectionHealth(ctx context.Context, name string, db *sql.DB, checkTime time.Time) modular.HealthReport {
	// Create base report structure
	report := modular.HealthReport{
		Module:        "database",
		Component:     name,
		CheckedAt:     checkTime,
		ObservedSince: checkTime,
		Optional:      false, // Database connections are not optional for readiness
		Details:       make(map[string]any),
	}

	// Test connectivity with ping
	if err := db.PingContext(ctx); err != nil {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = fmt.Sprintf("connection failed: %v", err)
		report.Details["ping_error"] = err.Error()
		report.Details["connection_name"] = name
		return report
	}

	// Get connection pool statistics for additional health information
	stats := db.Stats()
	report.Details["open_connections"] = stats.OpenConnections
	report.Details["in_use"] = stats.InUse
	report.Details["idle"] = stats.Idle
	report.Details["max_open_connections"] = stats.MaxOpenConnections
	report.Details["max_idle_connections"] = stats.MaxIdleClosed
	report.Details["connection_name"] = name

	// Determine health status based on connection statistics
	if stats.OpenConnections == 0 {
		report.Status = modular.HealthStatusUnhealthy
		report.Message = "no open connections in pool"
	} else if stats.MaxOpenConnections > 0 && float64(stats.OpenConnections)/float64(stats.MaxOpenConnections) > 0.9 {
		// If we're using more than 90% of max connections, consider it degraded
		report.Status = modular.HealthStatusDegraded
		report.Message = fmt.Sprintf("connection pool usage high: %d/%d connections", 
			stats.OpenConnections, stats.MaxOpenConnections)
	} else {
		report.Status = modular.HealthStatusHealthy
		report.Message = fmt.Sprintf("connection healthy: %d open connections", stats.OpenConnections)
	}

	// Add configuration details if available
	if m.config != nil {
		if connConfig, exists := m.config.Connections[name]; exists {
			report.Details["driver"] = connConfig.Driver
			report.Details["is_default"] = (name == m.config.Default)
		}
	}

	return report
}

// GetHealthTimeout returns the maximum time needed for health checks to complete.
// Database health checks typically involve network operations (ping), so we allow
// a reasonable timeout that accounts for potential network latency.
func (m *Module) GetHealthTimeout() time.Duration {
	// Base timeout for ping operations plus buffer for multiple connections
	baseTimeout := 5 * time.Second
	
	// Add additional time for each connection beyond the first
	if len(m.connections) > 1 {
		additionalTime := time.Duration(len(m.connections)-1) * 2 * time.Second
		return baseTimeout + additionalTime
	}
	
	return baseTimeout
}

// IsHealthy is a convenience method that returns true if all database connections
// are healthy. This is useful for quick health status checks without detailed reports.
func (m *Module) IsHealthy(ctx context.Context) bool {
	reports, err := m.HealthCheck(ctx)
	if err != nil {
		return false
	}
	
	for _, report := range reports {
		if report.Status != modular.HealthStatusHealthy {
			return false
		}
	}
	
	return true
}