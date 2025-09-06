package eventbus

import (
	"context"
	"testing"
	"time"

	modular "github.com/GoCodeAlone/modular" // root package for Subject and CloudEvent helpers
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// TestGetRegisteredEventTypes ensures the list is returned and stable length.
func TestGetRegisteredEventTypes(t *testing.T) {
	m := &EventBusModule{}
	types := m.GetRegisteredEventTypes()
	if len(types) != 10 { // keep in sync with module.go
		t.Fatalf("expected 10 event types, got %d", len(types))
	}
	// quick uniqueness check
	seen := map[string]struct{}{}
	for _, v := range types {
		if _, ok := seen[v]; ok {
			t.Fatalf("duplicate event type: %s", v)
		}
		seen[v] = struct{}{}
	}
}

// TestEmitEventNoSubject covers the silent skip path of emitEvent helper when no subject set.
func TestEmitEventNoSubject(t *testing.T) {
	m := &EventBusModule{}
	// No subject configured; should return immediately without panic.
	m.emitEvent(context.Background(), "eventbus.test.no_subject", map[string]interface{}{"k": "v"})
}

// TestEmitEventWithSubject exercises EmitEvent path including goroutine dispatch.
func TestEmitEventWithSubject(t *testing.T) {
	m := &EventBusModule{}
	subj := modularSubjectMock{}
	// set subject directly (simpler than full app wiring for coverage)
	m.mutex.Lock()
	m.subject = subj
	m.mutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := m.EmitEvent(ctx, modular.NewCloudEvent("eventbus.test.emit", "test", map[string]interface{}{"x": 1}, nil)); err != nil {
		t.Fatalf("EmitEvent returned error: %v", err)
	}
}

// modularSubjectMock implements minimal Subject interface needed for tests.
type modularSubjectMock struct{}

// Implement modular.Subject with minimal behavior
func (m modularSubjectMock) RegisterObserver(observer modular.Observer, eventTypes ...string) error {
	return nil
}
func (m modularSubjectMock) UnregisterObserver(observer modular.Observer) error { return nil }
func (m modularSubjectMock) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	return nil
}
func (m modularSubjectMock) GetObservers() []modular.ObserverInfo { return nil }
