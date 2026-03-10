package modular

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// --- Benchmark helpers ---

// benchModule is a minimal Module for bootstrap benchmarks.
type benchModule struct{ name string }

func (m *benchModule) Name() string             { return m.name }
func (m *benchModule) Init(_ Application) error { return nil }

// benchReloadable is a fast Reloadable for reload benchmarks.
type benchReloadable struct{ name string }

func (m *benchReloadable) Name() string             { return m.name }
func (m *benchReloadable) Init(_ Application) error { return nil }
func (m *benchReloadable) Reload(_ context.Context, _ []ConfigChange) error {
	return nil
}
func (m *benchReloadable) CanReload() bool              { return true }
func (m *benchReloadable) ReloadTimeout() time.Duration { return 5 * time.Second }

// benchLogger is a no-op logger for benchmarks.
type benchLogger struct{}

func (l *benchLogger) Info(_ string, _ ...any)  {}
func (l *benchLogger) Error(_ string, _ ...any) {}
func (l *benchLogger) Warn(_ string, _ ...any)  {}
func (l *benchLogger) Debug(_ string, _ ...any) {}

// BenchmarkBootstrap measures Init time with 10 modules. Target: <150ms.
func BenchmarkBootstrap(b *testing.B) {
	modules := make([]Module, 10)
	for i := range modules {
		modules[i] = &benchModule{name: fmt.Sprintf("bench-mod-%d", i)}
	}

	b.ResetTimer()
	for b.Loop() {
		app, err := NewApplication(
			WithLogger(&benchLogger{}),
			WithConfigProvider(NewStdConfigProvider(&struct{}{})),
			WithModules(modules...),
		)
		if err != nil {
			b.Fatalf("NewApplication failed: %v", err)
		}

		if err := app.Init(); err != nil {
			b.Fatalf("Init failed: %v", err)
		}
	}
}

// BenchmarkServiceLookup measures service registry lookup. Target: <2us.
func BenchmarkServiceLookup(b *testing.B) {
	registry := NewEnhancedServiceRegistry()
	_, _ = registry.RegisterService("bench-service", &struct{ Value int }{42})
	svcReg := registry.AsServiceRegistry()

	b.ResetTimer()
	for b.Loop() {
		_ = svcReg["bench-service"]
	}
}

// BenchmarkReload measures a single reload cycle with 5 modules. Target: <80ms.
func BenchmarkReload(b *testing.B) {
	log := &benchLogger{}
	orchestrator := NewReloadOrchestrator(log, nil)

	for i := 0; i < 5; i++ {
		mod := &benchReloadable{name: fmt.Sprintf("reload-mod-%d", i)}
		orchestrator.RegisterReloadable(mod.name, mod)
	}

	diff := ConfigDiff{
		Changed: map[string]FieldChange{
			"key1": {OldValue: "a", NewValue: "b", FieldPath: "key1", ChangeType: ChangeModified},
		},
		Added:   make(map[string]FieldChange),
		Removed: make(map[string]FieldChange),
	}

	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		req := ReloadRequest{
			Trigger: ReloadManual,
			Diff:    diff,
			Ctx:     ctx,
		}
		// Call processReload directly to measure the actual reload cycle
		// without channel/goroutine overhead.
		if err := orchestrator.processReload(ctx, req); err != nil {
			b.Fatalf("processReload failed: %v", err)
		}
	}
}

// BenchmarkHealthAggregation measures health check aggregation with 10 providers.
// Target: <5ms.
func BenchmarkHealthAggregation(b *testing.B) {
	svc := NewAggregateHealthService(WithCacheTTL(0))

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("provider-%d", i)
		provider := NewSimpleHealthProvider(name, "main", func(_ context.Context) (HealthStatus, string, error) {
			return StatusHealthy, "ok", nil
		})
		svc.AddProvider(name, provider)
	}

	// Force refresh on every call by using ForceHealthRefreshKey.
	ctx := context.WithValue(context.Background(), ForceHealthRefreshKey, true)

	b.ResetTimer()
	for b.Loop() {
		_, err := svc.Check(ctx)
		if err != nil {
			b.Fatalf("Check failed: %v", err)
		}
	}
}
