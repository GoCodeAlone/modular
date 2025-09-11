package modular

import (
	"context"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// fakeSubject captures events passed to NotifyObservers (simulating future integration)
type fakeSubject struct { // satisfies Subject
	mu        sync.Mutex
	events    []cloudevents.Event
	observers []Observer
}

func (f *fakeSubject) RegisterObserver(o Observer, _ ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observers = append(f.observers, o)
	return nil
}
func (f *fakeSubject) UnregisterObserver(o Observer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, ob := range f.observers {
		if ob == o {
			f.observers = append(f.observers[:i], f.observers[i+1:]...)
			break
		}
	}
	return nil
}
func (f *fakeSubject) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	f.mu.Lock()
	f.events = append(f.events, event)
	observers := append([]Observer(nil), f.observers...)
	f.mu.Unlock()
	for _, o := range observers {
		_ = o.OnEvent(ctx, event)
	}
	return nil
}
func (f *fakeSubject) GetObservers() []ObserverInfo { return nil }

// dynamicConfigSample used to test parseDynamicFields helper via reflection path.
type dynamicConfigSample struct {
	Name   string `dynamic:"true"`
	Nested struct {
		Value int `dynamic:"true"`
	}
	Static string
}

// TestParseDynamicFields_CoversRecursiveReflection ensures nested dynamic tag discovery.
func TestParseDynamicFields_CoversRecursiveReflection(t *testing.T) {
	cfg := dynamicConfigSample{}
	fields, err := parseDynamicFields(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 dynamic fields, got %d (%v)", len(fields), fields)
	}
}

// TestReloadOrchestrator_BackoffAndFailureEvents simulates consecutive failures to exercise backoff logic.
func TestReloadOrchestrator_BackoffAndFailureEvents(t *testing.T) {
	orch := NewReloadOrchestrator()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		orch.Stop(ctx)
	}()

	// Inject fake subject to allow event emission paths (even though current code placeholders skip Notify)
	fs := &fakeSubject{}
	orch.SetEventSubject(fs) // start/failed/success events early return check executed

	// Register a failing module to trigger failures and backoff state updates
	failModule := &testReloadModule{name: "fail", canReload: true, onReload: func(context.Context, []ConfigChange) error { return assertAnError }}
	// Minimal error sentinel
	orch.RegisterModule("fail", failModule)

	ctx := context.Background()
	// First failure
	_ = orch.RequestReload(ctx)
	if orch.failureCount != 1 {
		t.Fatalf("expected failureCount=1 got %d", orch.failureCount)
	}
	// Immediate second attempt should backoff or increment failure depending on timing
	_ = orch.RequestReload(ctx)
	if orch.failureCount < 1 {
		t.Fatalf("expected failureCount to remain >=1")
	}
}

// Sentinel error for module reload
var assertAnError = errTestFailure{}

type errTestFailure struct{}

func (e errTestFailure) Error() string { return "test failure" }

// TestReloadOrchestrator_NoopEvent emits noop event directly to cover emitNoopEvent branch.
func TestReloadOrchestrator_NoopEvent(t *testing.T) {
	orch := NewReloadOrchestrator()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		orch.Stop(ctx)
	}()
	fs := &fakeSubject{}
	orch.SetEventSubject(fs)
	// Directly call noop event method (currently placeholder)
	orch.emitNoopEvent("reload-noop-1", "no dynamic changes")
	// Allow goroutine to run
	time.Sleep(10 * time.Millisecond)
}
