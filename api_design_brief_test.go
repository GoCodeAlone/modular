package modular

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDesignBriefAPICompliance tests that our implementation matches the design brief APIs

func TestRequestReloadAPI(t *testing.T) {
	t.Run("RequestReload method exists and is callable", func(t *testing.T) {
		app := NewStdApplication(NewStdConfigProvider(struct{}{}), &briefTestLogger{t})
		
		// Should be callable without sections
		err := app.RequestReload()
		assert.Error(t, err) // Expected since it's not fully implemented yet
		assert.Contains(t, err.Error(), "not yet fully implemented")
		
		// Should be callable with sections
		err = app.RequestReload("section1", "section2")
		assert.Error(t, err) // Expected since it's not fully implemented yet
		assert.Contains(t, err.Error(), "not yet fully implemented")
	})
}

func TestRegisterHealthProviderAPI(t *testing.T) {
	t.Run("RegisterHealthProvider method exists and is callable", func(t *testing.T) {
		app := NewStdApplication(NewStdConfigProvider(struct{}{}), &briefTestLogger{t})
		
		provider := &testHealthProvider{
			module:    "test-module",
			component: "test-component",
			status:    HealthStatusHealthy,
		}
		
		// Should be callable with all parameters
		err := app.RegisterHealthProvider("test-module", provider, false)
		assert.NoError(t, err, "RegisterHealthProvider should succeed")
		
		// Should be callable with optional=true
		err = app.RegisterHealthProvider("test-module-optional", provider, true)
		assert.NoError(t, err, "RegisterHealthProvider with optional=true should succeed")
	})
}

func TestNewConfigChangeStructure(t *testing.T) {
	t.Run("ConfigChange struct has all required fields", func(t *testing.T) {
		change := ConfigChange{
			Section:   "database",
			FieldPath: "connection.host",
			OldValue:  "old-host",
			NewValue:  "new-host",
			Source:    "file:/config/app.yaml",
		}
		
		assert.Equal(t, "database", change.Section)
		assert.Equal(t, "connection.host", change.FieldPath)
		assert.Equal(t, "old-host", change.OldValue)
		assert.Equal(t, "new-host", change.NewValue)
		assert.Equal(t, "file:/config/app.yaml", change.Source)
	})
}

func TestNewHealthReportStructure(t *testing.T) {
	t.Run("HealthReport struct has all required fields", func(t *testing.T) {
		now := time.Now()
		observedSince := now.Add(-5 * time.Minute)
		
		report := HealthReport{
			Module:        "database",
			Component:     "connection-pool",
			Status:        HealthStatusHealthy,
			Message:       "All connections healthy",
			CheckedAt:     now,
			ObservedSince: observedSince,
			Optional:      false,
			Details: map[string]any{
				"active_connections": 10,
				"max_connections":    100,
			},
		}
		
		assert.Equal(t, "database", report.Module)
		assert.Equal(t, "connection-pool", report.Component)
		assert.Equal(t, HealthStatusHealthy, report.Status)
		assert.Equal(t, "All connections healthy", report.Message)
		assert.Equal(t, now, report.CheckedAt)
		assert.Equal(t, observedSince, report.ObservedSince)
		assert.False(t, report.Optional)
		assert.Equal(t, 10, report.Details["active_connections"])
		assert.Equal(t, 100, report.Details["max_connections"])
	})
}

func TestAggregatedHealthStructure(t *testing.T) {
	t.Run("AggregatedHealth struct has distinct readiness and health status", func(t *testing.T) {
		now := time.Now()
		
		reports := []HealthReport{
			{
				Module:        "database",
				Component:     "primary",
				Status:        HealthStatusHealthy,
				CheckedAt:     now,
				ObservedSince: now.Add(-time.Minute),
				Optional:      false,
			},
			{
				Module:        "cache",
				Component:     "redis",
				Status:        HealthStatusDegraded,
				CheckedAt:     now,
				ObservedSince: now.Add(-30 * time.Second),
				Optional:      true,
			},
		}
		
		aggregatedHealth := AggregatedHealth{
			Readiness:   HealthStatusHealthy, // Should be healthy because degraded component is optional
			Health:      HealthStatusDegraded, // Should reflect worst overall status
			Reports:     reports,
			GeneratedAt: now,
		}
		
		assert.Equal(t, HealthStatusHealthy, aggregatedHealth.Readiness)
		assert.Equal(t, HealthStatusDegraded, aggregatedHealth.Health)
		assert.Len(t, aggregatedHealth.Reports, 2)
		assert.Equal(t, now, aggregatedHealth.GeneratedAt)
	})
}

func TestEventNamesMatchDesignBrief(t *testing.T) {
	t.Run("Event constants match design brief specifications", func(t *testing.T) {
		// FR-045 Dynamic Reload events
		assert.Equal(t, "config.reload.start", EventTypeConfigReloadStart)
		assert.Equal(t, "config.reload.success", EventTypeConfigReloadSuccess)
		assert.Equal(t, "config.reload.failed", EventTypeConfigReloadFailed)
		assert.Equal(t, "config.reload.noop", EventTypeConfigReloadNoop)
		
		// FR-048 Health Aggregation events
		assert.Equal(t, "health.aggregate.updated", EventTypeHealthAggregateUpdated)
	})
}

func TestReloadableInterfaceUsesConfigChange(t *testing.T) {
	t.Run("New Reloadable interface uses []ConfigChange parameter", func(t *testing.T) {
		module := &testReloadableModuleForBrief{
			name:      "test-module",
			canReload: true,
			timeout:   30 * time.Second,
		}
		
		changes := []ConfigChange{
			{
				Section:   "test",
				FieldPath: "enabled",
				OldValue:  false,
				NewValue:  true,
				Source:    "test",
			},
		}
		
		err := module.Reload(context.Background(), changes)
		assert.NoError(t, err)
		assert.True(t, module.lastReloadCalled)
		assert.Len(t, module.lastChanges, 1)
		assert.Equal(t, "test", module.lastChanges[0].Section)
		assert.Equal(t, "enabled", module.lastChanges[0].FieldPath)
	})
}

func TestHealthProviderInterface(t *testing.T) {
	t.Run("New HealthProvider interface returns []HealthReport", func(t *testing.T) {
		provider := &testHealthProvider{
			module:    "test-module",
			component: "test-component",
			status:    HealthStatusHealthy,
		}
		
		reports, err := provider.HealthCheck(context.Background())
		assert.NoError(t, err)
		assert.Len(t, reports, 1)
		assert.Equal(t, "test-module", reports[0].Module)
		assert.Equal(t, "test-component", reports[0].Component)
		assert.Equal(t, HealthStatusHealthy, reports[0].Status)
	})
}

// Test helper implementations

type testHealthProvider struct {
	module    string
	component string
	status    HealthStatus
}

func (p *testHealthProvider) HealthCheck(ctx context.Context) ([]HealthReport, error) {
	return []HealthReport{
		{
			Module:        p.module,
			Component:     p.component,
			Status:        p.status,
			Message:       "Test health check",
			CheckedAt:     time.Now(),
			ObservedSince: time.Now().Add(-time.Minute),
			Optional:      false,
			Details:       map[string]any{"test": true},
		},
	}, nil
}

type testReloadableModuleForBrief struct {
	name              string
	canReload         bool
	timeout           time.Duration
	lastReloadCalled  bool
	lastChanges       []ConfigChange
}

func (m *testReloadableModuleForBrief) Reload(ctx context.Context, changes []ConfigChange) error {
	m.lastReloadCalled = true
	m.lastChanges = changes
	return nil
}

func (m *testReloadableModuleForBrief) CanReload() bool {
	return m.canReload
}

func (m *testReloadableModuleForBrief) ReloadTimeout() time.Duration {
	return m.timeout
}

type briefTestLogger struct {
	t *testing.T
}

func (l *briefTestLogger) Debug(msg string, keyvals ...interface{}) {
	l.t.Logf("DEBUG: %s %v", msg, keyvals)
}

func (l *briefTestLogger) Info(msg string, keyvals ...interface{}) {
	l.t.Logf("INFO: %s %v", msg, keyvals)
}

func (l *briefTestLogger) Warn(msg string, keyvals ...interface{}) {
	l.t.Logf("WARN: %s %v", msg, keyvals)
}

func (l *briefTestLogger) Error(msg string, keyvals ...interface{}) {
	l.t.Logf("ERROR: %s %v", msg, keyvals)
}