package modular

import (
	"slices"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// mockReloadable is a test double for the Reloadable interface.
type mockReloadable struct {
	canReload   bool
	timeout     time.Duration
	reloadErr   error
	reloadCalls atomic.Int32
	lastChanges []ConfigChange
	mu          sync.Mutex
}

func (m *mockReloadable) Reload(_ context.Context, changes []ConfigChange) error {
	m.reloadCalls.Add(1)
	m.mu.Lock()
	m.lastChanges = changes
	m.mu.Unlock()
	return m.reloadErr
}

func (m *mockReloadable) CanReload() bool              { return m.canReload }
func (m *mockReloadable) ReloadTimeout() time.Duration { return m.timeout }

func (m *mockReloadable) getLastChanges() []ConfigChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ConfigChange, len(m.lastChanges))
	copy(result, m.lastChanges)
	return result
}

// reloadTestLogger implements Logger for testing.
type reloadTestLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *reloadTestLogger) Info(msg string, args ...any)  { l.record("INFO", msg, args...) }
func (l *reloadTestLogger) Error(msg string, args ...any) { l.record("ERROR", msg, args...) }
func (l *reloadTestLogger) Warn(msg string, args ...any)  { l.record("WARN", msg, args...) }
func (l *reloadTestLogger) Debug(msg string, args ...any) { l.record("DEBUG", msg, args...) }

func (l *reloadTestLogger) record(level, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf("[%s] %s %v", level, msg, args))
}

// reloadTestSubject is a minimal Subject for capturing events in reload tests.
type reloadTestSubject struct {
	mu     sync.Mutex
	events []cloudevents.Event
}

func (s *reloadTestSubject) RegisterObserver(_ Observer, _ ...string) error { return nil }
func (s *reloadTestSubject) UnregisterObserver(_ Observer) error            { return nil }
func (s *reloadTestSubject) GetObservers() []ObserverInfo                   { return nil }
func (s *reloadTestSubject) NotifyObservers(_ context.Context, event cloudevents.Event) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *reloadTestSubject) getEvents() []cloudevents.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]cloudevents.Event, len(s.events))
	copy(result, s.events)
	return result
}

func (s *reloadTestSubject) eventTypes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var types []string
	for _, e := range s.events {
		types = append(types, e.Type())
	}
	return types
}

// --- ConfigDiff tests ---

func TestConfigDiff_HasChanges(t *testing.T) {
	t.Run("empty diff", func(t *testing.T) {
		d := ConfigDiff{
			Changed: make(map[string]FieldChange),
			Added:   make(map[string]FieldChange),
			Removed: make(map[string]FieldChange),
		}
		if d.HasChanges() {
			t.Error("expected no changes")
		}
	})
	t.Run("with changed", func(t *testing.T) {
		d := ConfigDiff{
			Changed: map[string]FieldChange{"a": {OldValue: 1, NewValue: 2}},
			Added:   make(map[string]FieldChange),
			Removed: make(map[string]FieldChange),
		}
		if !d.HasChanges() {
			t.Error("expected changes")
		}
	})
	t.Run("with added", func(t *testing.T) {
		d := ConfigDiff{
			Changed: make(map[string]FieldChange),
			Added:   map[string]FieldChange{"b": {NewValue: "x"}},
			Removed: make(map[string]FieldChange),
		}
		if !d.HasChanges() {
			t.Error("expected changes")
		}
	})
	t.Run("with removed", func(t *testing.T) {
		d := ConfigDiff{
			Changed: make(map[string]FieldChange),
			Added:   make(map[string]FieldChange),
			Removed: map[string]FieldChange{"c": {OldValue: "y"}},
		}
		if !d.HasChanges() {
			t.Error("expected changes")
		}
	})
}

func TestConfigDiff_FilterByPrefix(t *testing.T) {
	d := ConfigDiff{
		Changed: map[string]FieldChange{
			"db.host":   {OldValue: "old", NewValue: "new"},
			"db.port":   {OldValue: 3306, NewValue: 5432},
			"cache.ttl": {OldValue: 30, NewValue: 60},
		},
		Added: map[string]FieldChange{
			"db.ssl": {NewValue: true},
		},
		Removed: map[string]FieldChange{
			"cache.max": {OldValue: 100},
		},
	}

	filtered := d.FilterByPrefix("db.")
	if len(filtered.Changed) != 2 {
		t.Errorf("expected 2 changed, got %d", len(filtered.Changed))
	}
	if len(filtered.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(filtered.Added))
	}
	if len(filtered.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(filtered.Removed))
	}

	cacheFiltered := d.FilterByPrefix("cache.")
	if len(cacheFiltered.Changed) != 1 {
		t.Errorf("expected 1 changed for cache prefix, got %d", len(cacheFiltered.Changed))
	}
	if len(cacheFiltered.Removed) != 1 {
		t.Errorf("expected 1 removed for cache prefix, got %d", len(cacheFiltered.Removed))
	}
}

func TestConfigDiff_RedactSensitiveFields(t *testing.T) {
	d := ConfigDiff{
		Changed: map[string]FieldChange{
			"db.password": {OldValue: "secret1", NewValue: "secret2", IsSensitive: true},
			"db.host":     {OldValue: "old", NewValue: "new", IsSensitive: false},
		},
		Added:   make(map[string]FieldChange),
		Removed: make(map[string]FieldChange),
	}

	redacted := d.RedactSensitiveFields()

	pw := redacted.Changed["db.password"]
	if pw.OldValue != "[REDACTED]" || pw.NewValue != "[REDACTED]" {
		t.Errorf("sensitive field not redacted: old=%v new=%v", pw.OldValue, pw.NewValue)
	}

	host := redacted.Changed["db.host"]
	if host.OldValue != "old" || host.NewValue != "new" {
		t.Errorf("non-sensitive field should not be redacted: old=%v new=%v", host.OldValue, host.NewValue)
	}

	// Verify original is not mutated.
	origPw := d.Changed["db.password"]
	if origPw.OldValue != "secret1" {
		t.Error("original diff should not be mutated")
	}
}

func TestConfigDiff_ChangeSummary(t *testing.T) {
	t.Run("no changes", func(t *testing.T) {
		d := ConfigDiff{
			Changed: make(map[string]FieldChange),
			Added:   make(map[string]FieldChange),
			Removed: make(map[string]FieldChange),
		}
		s := d.ChangeSummary()
		if s != "no changes" {
			t.Errorf("expected 'no changes', got %q", s)
		}
	})
	t.Run("mixed changes", func(t *testing.T) {
		d := ConfigDiff{
			Changed: map[string]FieldChange{"a": {}},
			Added:   map[string]FieldChange{"b": {}, "c": {}},
			Removed: map[string]FieldChange{"d": {}},
		}
		s := d.ChangeSummary()
		if !strings.Contains(s, "2 added") {
			t.Errorf("summary missing added count: %q", s)
		}
		if !strings.Contains(s, "1 modified") {
			t.Errorf("summary missing modified count: %q", s)
		}
		if !strings.Contains(s, "1 removed") {
			t.Errorf("summary missing removed count: %q", s)
		}
	})
}

// waitFor polls cond every 5ms until it returns true or timeout elapses.
// Returns true if cond was satisfied, false on timeout.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// --- ReloadOrchestrator tests ---

func newTestDiff() ConfigDiff {
	return ConfigDiff{
		Changed: map[string]FieldChange{
			"db.host": {OldValue: "localhost", NewValue: "remotehost", ChangeType: ChangeModified},
		},
		Added:     make(map[string]FieldChange),
		Removed:   make(map[string]FieldChange),
		Timestamp: time.Now(),
		DiffID:    "test-diff-1",
	}
}

func TestReloadOrchestrator_SuccessfulReload(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	mod := &mockReloadable{canReload: true, timeout: 5 * time.Second}
	orch.RegisterReloadable("testmod", mod)

	ctx := t.Context()
	orch.Start(ctx)

	diff := newTestDiff()
	if err := orch.RequestReload(ctx, ReloadManual, diff); err != nil {
		t.Fatalf("RequestReload failed: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool { return mod.reloadCalls.Load() >= 1 }) {
		t.Fatalf("timed out waiting for reload call, got %d", mod.reloadCalls.Load())
	}

	if !waitFor(t, 2*time.Second, func() bool { return len(subject.eventTypes()) >= 2 }) {
		t.Fatalf("timed out waiting for events, got %d", len(subject.eventTypes()))
	}

	events := subject.eventTypes()
	if events[0] != EventTypeConfigReloadStarted {
		t.Errorf("expected started event, got %s", events[0])
	}
	if events[len(events)-1] != EventTypeConfigReloadCompleted {
		t.Errorf("expected completed event, got %s", events[len(events)-1])
	}
}

func TestReloadOrchestrator_PartialFailure_Rollback(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	mod1 := &mockReloadable{canReload: true, timeout: 5 * time.Second}
	mod2 := &mockReloadable{canReload: true, timeout: 5 * time.Second, reloadErr: errors.New("boom")}
	orch.RegisterReloadable("aaa_first", mod1)
	orch.RegisterReloadable("zzz_second", mod2)

	ctx := t.Context()
	orch.Start(ctx)

	diff := newTestDiff()
	if err := orch.RequestReload(ctx, ReloadManual, diff); err != nil {
		t.Fatalf("RequestReload failed: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool {
		return len(subject.eventTypes()) > 0 && subject.eventTypes()[len(subject.eventTypes())-1] == EventTypeConfigReloadFailed
	}) {
		t.Fatal("timed out waiting for reload failure event")
	}

	// Targets are sorted by name: aaa_first runs before zzz_second.
	// aaa_first succeeds, then zzz_second fails, triggering rollback of aaa_first.
	// So aaa_first gets 2 calls (apply + rollback) and zzz_second gets 1 call (the failure).
	calls1 := mod1.reloadCalls.Load()
	calls2 := mod2.reloadCalls.Load()

	if calls1 != 2 {
		t.Errorf("expected aaa_first to be called 2 times (apply+rollback), got %d", calls1)
	}

	if calls2 != 1 {
		t.Errorf("expected zzz_second to be called 1 time (the failure), got %d", calls2)
	}

	// Verify a failed event was emitted.
	hasFailedEvent := false
	for _, et := range subject.eventTypes() {
		if et == EventTypeConfigReloadFailed {
			hasFailedEvent = true
		}
	}
	if !hasFailedEvent {
		t.Error("expected ConfigReloadFailed event")
	}
}

func TestReloadOrchestrator_CircuitBreaker(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	failMod := &mockReloadable{canReload: true, timeout: 5 * time.Second, reloadErr: errors.New("fail")}
	orch.RegisterReloadable("failing", failMod)

	ctx := t.Context()
	orch.Start(ctx)

	diff := newTestDiff()

	// Trigger enough failures to open the circuit breaker.
	for i := range circuitBreakerThreshold {
		if err := orch.RequestReload(ctx, ReloadManual, diff); err != nil {
			t.Fatalf("RequestReload %d failed: %v", i, err)
		}
		expected := int32(i + 1)
		if !waitFor(t, 2*time.Second, func() bool { return failMod.reloadCalls.Load() >= expected }) {
			t.Fatalf("timed out waiting for reload call %d", i+1)
		}
	}

	// Next request should be rejected by the circuit breaker.
	err := orch.RequestReload(ctx, ReloadManual, diff)
	if err == nil {
		t.Error("expected circuit breaker error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "circuit breaker") {
		t.Errorf("expected circuit breaker error, got: %v", err)
	}
}

func TestReloadOrchestrator_CanReloadFalse_Skipped(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	mod := &mockReloadable{canReload: false, timeout: 5 * time.Second}
	orch.RegisterReloadable("disabled", mod)

	ctx := t.Context()
	orch.Start(ctx)

	diff := newTestDiff()
	if err := orch.RequestReload(ctx, ReloadManual, diff); err != nil {
		t.Fatalf("RequestReload failed: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool {
		return slices.Contains(subject.eventTypes(), EventTypeConfigReloadCompleted)
	}) {
		t.Fatal("timed out waiting for ConfigReloadCompleted event")
	}

	if mod.reloadCalls.Load() != 0 {
		t.Errorf("expected 0 reload calls for disabled module, got %d", mod.reloadCalls.Load())
	}
}

func TestReloadOrchestrator_ConcurrentRequests(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	mod := &mockReloadable{canReload: true, timeout: 5 * time.Second}
	orch.RegisterReloadable("concurrent", mod)

	ctx := t.Context()
	orch.Start(ctx)

	diff := newTestDiff()

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = orch.RequestReload(ctx, ReloadManual, diff)
		})
	}
	wg.Wait()

	if !waitFor(t, 2*time.Second, func() bool { return mod.reloadCalls.Load() >= 1 }) {
		t.Fatalf("timed out waiting for at least 1 reload call, got %d", mod.reloadCalls.Load())
	}

	calls := mod.reloadCalls.Load()
	// Due to single-flight, some may be skipped — that's expected.
	t.Logf("concurrent test: %d reload calls processed out of 10 requests", calls)
}

func TestReloadOrchestrator_NoopOnEmptyDiff(t *testing.T) {
	logger := &reloadTestLogger{}
	subject := &reloadTestSubject{}
	orch := NewReloadOrchestrator(logger, subject)

	mod := &mockReloadable{canReload: true, timeout: 5 * time.Second}
	orch.RegisterReloadable("mod", mod)

	ctx := t.Context()
	orch.Start(ctx)

	emptyDiff := ConfigDiff{
		Changed: make(map[string]FieldChange),
		Added:   make(map[string]FieldChange),
		Removed: make(map[string]FieldChange),
		DiffID:  "empty",
	}
	if err := orch.RequestReload(ctx, ReloadManual, emptyDiff); err != nil {
		t.Fatalf("RequestReload failed: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool {
		return slices.Contains(subject.eventTypes(), EventTypeConfigReloadNoop)
	}) {
		t.Fatal("timed out waiting for ConfigReloadNoop event")
	}

	if mod.reloadCalls.Load() != 0 {
		t.Errorf("expected 0 reload calls for empty diff, got %d", mod.reloadCalls.Load())
	}
}
