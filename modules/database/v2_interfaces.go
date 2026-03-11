package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Compile-time interface assertions.
var (
	_ modular.MetricsProvider = (*Module)(nil)
	_ modular.Drainable       = (*Module)(nil)
	_ modular.Reloadable      = (*Module)(nil)
)

// CollectMetrics implements modular.MetricsProvider.
// It returns pool statistics from sql.DBStats for every connection.
func (m *Module) CollectMetrics(_ context.Context) modular.ModuleMetrics {
	values := make(map[string]float64)

	multipleConns := len(m.connections) > 1

	for name, db := range m.connections {
		stats := db.Stats()
		prefix := ""
		if multipleConns {
			prefix = name + "."
		}
		values[prefix+"open_connections"] = float64(stats.OpenConnections)
		values[prefix+"in_use"] = float64(stats.InUse)
		values[prefix+"idle"] = float64(stats.Idle)
		values[prefix+"wait_count"] = float64(stats.WaitCount)
		values[prefix+"wait_duration_ms"] = float64(stats.WaitDuration.Milliseconds())
		values[prefix+"max_open"] = float64(stats.MaxOpenConnections)
	}

	return modular.ModuleMetrics{
		Name:   Name,
		Values: values,
	}
}

// PreStop implements modular.Drainable.
// It sets max open connections to 0 on all connections to prevent new connections,
// then waits briefly for active queries to finish.
func (m *Module) PreStop(ctx context.Context) error {
	m.logger.Info("Draining database connections", "count", len(m.connections))

	for name, db := range m.connections {
		db.SetMaxOpenConns(0)
		m.logger.Info("Set max open connections to 0", "connection", name)
	}

	// Wait for active queries to finish, respecting context deadline.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		allIdle := true
		for _, db := range m.connections {
			if db.Stats().InUse > 0 {
				allIdle = false
				break
			}
		}
		if allIdle {
			m.logger.Info("All database connections drained")
			return nil
		}

		select {
		case <-ctx.Done():
			m.logger.Warn("Drain timeout reached, proceeding with active connections")
			return nil
		case <-ticker.C:
			// Check again
		}
	}
}

// CanReload implements modular.Reloadable.
func (m *Module) CanReload() bool {
	return true
}

// ReloadTimeout implements modular.Reloadable.
func (m *Module) ReloadTimeout() time.Duration {
	return 10 * time.Second
}

// Reload implements modular.Reloadable.
// It applies pool configuration changes to existing connections without reconnecting.
func (m *Module) Reload(_ context.Context, changes []modular.ConfigChange) error {
	for _, change := range changes {
		// Match field paths like "MaxOpenConnections" or "connections.<name>.MaxOpenConnections"
		field := change.FieldPath
		parts := strings.Split(field, ".")

		// Determine target field name (last segment)
		targetField := parts[len(parts)-1]

		// Determine which connections to apply to
		var targetConns []string
		if len(parts) >= 3 && parts[0] == "connections" {
			// Scoped to a specific connection: connections.<name>.<field>
			targetConns = []string{parts[1]}
		} else {
			// Apply to all connections
			for name := range m.connections {
				targetConns = append(targetConns, name)
			}
		}

		for _, connName := range targetConns {
			db, ok := m.connections[connName]
			if !ok {
				continue
			}

			switch targetField {
			case "MaxOpenConnections":
				if v, err := strconv.Atoi(change.NewValue); err == nil {
					db.SetMaxOpenConns(v)
					m.logger.Info("Reloaded MaxOpenConnections", "connection", connName, "value", v)
				}
			case "MaxIdleConnections":
				if v, err := strconv.Atoi(change.NewValue); err == nil {
					db.SetMaxIdleConns(v)
					m.logger.Info("Reloaded MaxIdleConnections", "connection", connName, "value", v)
				}
			case "ConnectionMaxLifetime":
				if v, err := time.ParseDuration(change.NewValue); err == nil {
					db.SetConnMaxLifetime(v)
					m.logger.Info("Reloaded ConnectionMaxLifetime", "connection", connName, "value", v)
				}
			case "ConnectionMaxIdleTime":
				if v, err := time.ParseDuration(change.NewValue); err == nil {
					db.SetConnMaxIdleTime(v)
					m.logger.Info("Reloaded ConnectionMaxIdleTime", "connection", connName, "value", v)
				}
			default:
				m.logger.Debug("Ignoring unrecognized config change", "field", fmt.Sprintf("%s (target: %s)", field, targetField))
			}
		}
	}
	return nil
}
