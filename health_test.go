package modular

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status HealthStatus
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
		{HealthStatus(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("HealthStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestHealthStatus_IsHealthy(t *testing.T) {
	if !StatusHealthy.IsHealthy() {
		t.Error("StatusHealthy.IsHealthy() should be true")
	}
	for _, s := range []HealthStatus{StatusUnknown, StatusDegraded, StatusUnhealthy} {
		if s.IsHealthy() {
			t.Errorf("%v.IsHealthy() should be false", s)
		}
	}
}

func TestSimpleHealthProvider(t *testing.T) {
	provider := NewSimpleHealthProvider("mymod", "db", func(_ context.Context) (HealthStatus, string, error) {
		return StatusHealthy, "all good", nil
	})
	reports, err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	r := reports[0]
	if r.Module != "mymod" || r.Component != "db" || r.Status != StatusHealthy || r.Message != "all good" {
		t.Errorf("unexpected report: %+v", r)
	}
	if r.CheckedAt.IsZero() {
		t.Error("CheckedAt should be set")
	}
}

func TestStaticHealthProvider(t *testing.T) {
	report := HealthReport{
		Module:    "static",
		Component: "cache",
		Status:    StatusDegraded,
		Message:   "warming up",
	}
	provider := NewStaticHealthProvider(report)
	reports, err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].Status != StatusDegraded {
		t.Errorf("expected degraded, got %v", reports[0].Status)
	}
	if reports[0].CheckedAt.IsZero() {
		t.Error("CheckedAt should be set by static provider")
	}
}

func TestCompositeHealthProvider(t *testing.T) {
	p1 := NewStaticHealthProvider(HealthReport{Module: "a", Component: "1", Status: StatusHealthy})
	p2 := NewStaticHealthProvider(HealthReport{Module: "b", Component: "2", Status: StatusDegraded})
	composite := NewCompositeHealthProvider(p1, p2)

	reports, err := composite.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
}

// testSubject is a minimal Subject implementation for testing event emission.
type testSubject struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func (s *testSubject) RegisterObserver(_ Observer, _ ...string) error   { return nil }
func (s *testSubject) UnregisterObserver(_ Observer) error               { return nil }
func (s *testSubject) GetObservers() []ObserverInfo                      { return nil }
func (s *testSubject) NotifyObservers(_ context.Context, event cloudevents.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}
func (s *testSubject) getEvents() []cloudevents.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]cloudevents.Event, len(s.events))
	copy(result, s.events)
	return result
}

func TestAggregateHealthService_SingleProvider(t *testing.T) {
	svc := NewAggregateHealthService()
	svc.AddProvider("db", NewStaticHealthProvider(HealthReport{
		Module: "db", Component: "conn", Status: StatusHealthy, Message: "ok",
	}))

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Health != StatusHealthy {
		t.Errorf("expected healthy, got %v", result.Health)
	}
	if result.Readiness != StatusHealthy {
		t.Errorf("expected readiness healthy, got %v", result.Readiness)
	}
	if len(result.Reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(result.Reports))
	}
}

func TestAggregateHealthService_MultipleProviders(t *testing.T) {
	svc := NewAggregateHealthService()
	svc.AddProvider("db", NewStaticHealthProvider(HealthReport{
		Module: "db", Component: "conn", Status: StatusHealthy,
	}))
	svc.AddProvider("cache", NewStaticHealthProvider(HealthReport{
		Module: "cache", Component: "redis", Status: StatusDegraded,
	}))

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Health != StatusDegraded {
		t.Errorf("expected degraded health, got %v", result.Health)
	}
	if result.Readiness != StatusDegraded {
		t.Errorf("expected degraded readiness, got %v", result.Readiness)
	}
	if len(result.Reports) != 2 {
		t.Errorf("expected 2 reports, got %d", len(result.Reports))
	}
}

func TestAggregateHealthService_OptionalVsRequired(t *testing.T) {
	svc := NewAggregateHealthService()
	svc.AddProvider("db", NewStaticHealthProvider(HealthReport{
		Module: "db", Component: "conn", Status: StatusHealthy,
	}))
	svc.AddProvider("metrics", NewStaticHealthProvider(HealthReport{
		Module: "metrics", Component: "export", Status: StatusUnhealthy, Optional: true,
	}))

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Health reflects all components (worst = unhealthy)
	if result.Health != StatusUnhealthy {
		t.Errorf("expected unhealthy health (includes optional), got %v", result.Health)
	}
	// Readiness only reflects required components (should be healthy)
	if result.Readiness != StatusHealthy {
		t.Errorf("expected healthy readiness (optional excluded), got %v", result.Readiness)
	}
}

func TestAggregateHealthService_CacheHit(t *testing.T) {
	callCount := 0
	provider := NewSimpleHealthProvider("mod", "comp", func(_ context.Context) (HealthStatus, string, error) {
		callCount++
		return StatusHealthy, "ok", nil
	})

	svc := NewAggregateHealthService(WithCacheTTL(1 * time.Second))
	svc.AddProvider("test", provider)

	// First call
	_, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call within TTL should be cached
	_, err = svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (cached), got %d", callCount)
	}
}

func TestAggregateHealthService_CacheMiss(t *testing.T) {
	callCount := 0
	provider := NewSimpleHealthProvider("mod", "comp", func(_ context.Context) (HealthStatus, string, error) {
		callCount++
		return StatusHealthy, "ok", nil
	})

	svc := NewAggregateHealthService(WithCacheTTL(1 * time.Millisecond))
	svc.AddProvider("test", provider)

	_, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	_, err = svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls after cache expiry, got %d", callCount)
	}
}

func TestAggregateHealthService_CacheInvalidation(t *testing.T) {
	callCount := 0
	provider := NewSimpleHealthProvider("mod", "comp", func(_ context.Context) (HealthStatus, string, error) {
		callCount++
		return StatusHealthy, "ok", nil
	})

	svc := NewAggregateHealthService(WithCacheTTL(10 * time.Second))
	svc.AddProvider("test", provider)

	_, _ = svc.Check(context.Background())
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// AddProvider should invalidate cache
	svc.AddProvider("another", NewStaticHealthProvider(HealthReport{
		Module: "x", Component: "y", Status: StatusHealthy,
	}))

	_, _ = svc.Check(context.Background())
	if callCount != 2 {
		t.Errorf("expected 2 calls after AddProvider invalidation, got %d", callCount)
	}

	// RemoveProvider should also invalidate
	svc.RemoveProvider("another")
	_, _ = svc.Check(context.Background())
	if callCount != 3 {
		t.Errorf("expected 3 calls after RemoveProvider invalidation, got %d", callCount)
	}
}

func TestAggregateHealthService_ForceRefresh(t *testing.T) {
	callCount := 0
	provider := NewSimpleHealthProvider("mod", "comp", func(_ context.Context) (HealthStatus, string, error) {
		callCount++
		return StatusHealthy, "ok", nil
	})

	svc := NewAggregateHealthService(WithCacheTTL(10 * time.Second))
	svc.AddProvider("test", provider)

	_, _ = svc.Check(context.Background())
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Force refresh bypasses cache
	ctx := context.WithValue(context.Background(), ForceHealthRefreshKey, true)
	_, _ = svc.Check(ctx)
	if callCount != 2 {
		t.Errorf("expected 2 calls after force refresh, got %d", callCount)
	}
}

func TestAggregateHealthService_PanicRecovery(t *testing.T) {
	panicProvider := NewSimpleHealthProvider("panicky", "boom", func(_ context.Context) (HealthStatus, string, error) {
		panic("something went wrong")
	})

	svc := NewAggregateHealthService()
	svc.AddProvider("panicky", panicProvider)

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Health != StatusUnhealthy {
		t.Errorf("expected unhealthy after panic, got %v", result.Health)
	}
	// Check that the panic report is present
	found := false
	for _, r := range result.Reports {
		if r.Status == StatusUnhealthy && r.Component == "panic-recovery" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected panic recovery report in results")
	}
}

// temporaryError implements the Temporary() interface.
type temporaryError struct {
	msg string
}

func (e *temporaryError) Error() string   { return e.msg }
func (e *temporaryError) Temporary() bool { return true }

func TestAggregateHealthService_TemporaryError(t *testing.T) {
	provider := NewSimpleHealthProvider("net", "conn", func(_ context.Context) (HealthStatus, string, error) {
		return StatusUnknown, "", &temporaryError{msg: "connection timeout"}
	})

	svc := NewAggregateHealthService()
	svc.AddProvider("net", provider)

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Health != StatusDegraded {
		t.Errorf("expected degraded for temporary error, got %v", result.Health)
	}
}

func TestAggregateHealthService_PermanentError(t *testing.T) {
	provider := NewSimpleHealthProvider("db", "conn", func(_ context.Context) (HealthStatus, string, error) {
		return StatusUnknown, "", errors.New("connection refused")
	})

	svc := NewAggregateHealthService()
	svc.AddProvider("db", provider)

	result, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Health != StatusUnhealthy {
		t.Errorf("expected unhealthy for permanent error, got %v", result.Health)
	}
}

func TestAggregateHealthService_EventEmission(t *testing.T) {
	sub := &testSubject{}
	svc := NewAggregateHealthService(WithSubject(sub))
	svc.AddProvider("db", NewStaticHealthProvider(HealthReport{
		Module: "db", Component: "conn", Status: StatusHealthy,
	}))

	_, _ = svc.Check(context.Background())

	events := sub.getEvents()
	// First check: should emit evaluated + status changed (unknown -> healthy)
	if len(events) < 1 {
		t.Fatal("expected at least 1 event")
	}

	hasEvaluated := false
	hasChanged := false
	for _, e := range events {
		switch e.Type() {
		case EventTypeHealthEvaluated:
			hasEvaluated = true
		case EventTypeHealthStatusChanged:
			hasChanged = true
		}
	}
	if !hasEvaluated {
		t.Error("expected health evaluated event")
	}
	if !hasChanged {
		t.Error("expected health status changed event (unknown -> healthy)")
	}
}

func TestAggregateHealthService_ConcurrentChecks(t *testing.T) {
	svc := NewAggregateHealthService(WithCacheTTL(1 * time.Millisecond))
	svc.AddProvider("db", NewStaticHealthProvider(HealthReport{
		Module: "db", Component: "conn", Status: StatusHealthy,
	}))

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.Check(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if result == nil {
				errs <- errors.New("nil result")
				return
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent check error: %v", err)
	}
}
