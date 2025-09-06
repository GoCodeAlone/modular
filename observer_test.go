package modular

import (
	"context"
	"errors"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

func TestCloudEvent(t *testing.T) {
	t.Parallel()
	metadata := map[string]interface{}{"key": "value"}
	event := NewCloudEvent(
		"test.event",
		"test.source",
		"test data",
		metadata,
	)

	if event.Type() != "test.event" {
		t.Errorf("Expected Type to be 'test.event', got %s", event.Type())
	}
	if event.Source() != "test.source" {
		t.Errorf("Expected Source to be 'test.source', got %s", event.Source())
	}

	// Check data
	var data string
	if err := event.DataAs(&data); err != nil {
		t.Errorf("Failed to extract data: %v", err)
	}
	if data != "test data" {
		t.Errorf("Expected Data to be 'test data', got %v", data)
	}

	// Check extension
	if val, ok := event.Extensions()["key"]; !ok || val != "value" {
		t.Errorf("Expected Extension['key'] to be 'value', got %v", val)
	}
}

func TestFunctionalObserver(t *testing.T) {
	t.Parallel()
	called := false
	var receivedEvent cloudevents.Event

	handler := func(ctx context.Context, event cloudevents.Event) error {
		called = true
		receivedEvent = event
		return nil
	}

	observer := NewFunctionalObserver("test-observer", handler)

	// Test ObserverID
	if observer.ObserverID() != "test-observer" {
		t.Errorf("Expected ObserverID to be 'test-observer', got %s", observer.ObserverID())
	}

	// Test OnEvent
	testEvent := NewCloudEvent(
		"test.event",
		"test",
		"test data",
		nil,
	)

	err := observer.OnEvent(context.Background(), testEvent)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected handler to be called")
	}

	if receivedEvent.Type() != testEvent.Type() {
		t.Errorf("Expected received event type to be %s, got %s", testEvent.Type(), receivedEvent.Type())
	}
}

var errTest = errors.New("test error")

func TestFunctionalObserverWithError(t *testing.T) {
	t.Parallel()
	expectedErr := errTest

	handler := func(ctx context.Context, event cloudevents.Event) error {
		return expectedErr
	}

	observer := NewFunctionalObserver("test-observer", handler)

	testEvent := NewCloudEvent(
		"test.event",
		"test",
		"test data",
		nil,
	)

	err := observer.OnEvent(context.Background(), testEvent)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestEventTypeConstants(t *testing.T) {
	t.Parallel()
	// Test that our event type constants are properly defined with reverse domain notation
	expectedEventTypes := map[string]string{
		"EventTypeModuleRegistered":    "com.modular.module.registered",
		"EventTypeModuleInitialized":   "com.modular.module.initialized",
		"EventTypeModuleStarted":       "com.modular.module.started",
		"EventTypeModuleStopped":       "com.modular.module.stopped",
		"EventTypeModuleFailed":        "com.modular.module.failed",
		"EventTypeServiceRegistered":   "com.modular.service.registered",
		"EventTypeServiceUnregistered": "com.modular.service.unregistered",
		"EventTypeServiceRequested":    "com.modular.service.requested",
		"EventTypeConfigLoaded":        "com.modular.config.loaded",
		"EventTypeConfigValidated":     "com.modular.config.validated",
		"EventTypeConfigChanged":       "com.modular.config.changed",
		"EventTypeApplicationStarted":  "com.modular.application.started",
		"EventTypeApplicationStopped":  "com.modular.application.stopped",
		"EventTypeApplicationFailed":   "com.modular.application.failed",
	}

	actualEventTypes := map[string]string{
		"EventTypeModuleRegistered":    EventTypeModuleRegistered,
		"EventTypeModuleInitialized":   EventTypeModuleInitialized,
		"EventTypeModuleStarted":       EventTypeModuleStarted,
		"EventTypeModuleStopped":       EventTypeModuleStopped,
		"EventTypeModuleFailed":        EventTypeModuleFailed,
		"EventTypeServiceRegistered":   EventTypeServiceRegistered,
		"EventTypeServiceUnregistered": EventTypeServiceUnregistered,
		"EventTypeServiceRequested":    EventTypeServiceRequested,
		"EventTypeConfigLoaded":        EventTypeConfigLoaded,
		"EventTypeConfigValidated":     EventTypeConfigValidated,
		"EventTypeConfigChanged":       EventTypeConfigChanged,
		"EventTypeApplicationStarted":  EventTypeApplicationStarted,
		"EventTypeApplicationStopped":  EventTypeApplicationStopped,
		"EventTypeApplicationFailed":   EventTypeApplicationFailed,
	}

	for name, expected := range expectedEventTypes {
		if actual, exists := actualEventTypes[name]; !exists {
			t.Errorf("Event type constant %s is not defined", name)
		} else if actual != expected {
			t.Errorf("Event type constant %s has value %s, expected %s", name, actual, expected)
		}
	}
}

// Mock implementation for testing Subject interface
type mockSubject struct {
	observers map[string]*mockObserverRegistration
	events    []cloudevents.Event
}

type mockObserverRegistration struct {
	observer   Observer
	eventTypes []string
	registered time.Time
}

func newMockSubject() *mockSubject {
	return &mockSubject{
		observers: make(map[string]*mockObserverRegistration),
		events:    make([]cloudevents.Event, 0),
	}
}

func (m *mockSubject) RegisterObserver(observer Observer, eventTypes ...string) error {
	m.observers[observer.ObserverID()] = &mockObserverRegistration{
		observer:   observer,
		eventTypes: eventTypes,
		registered: time.Now(),
	}
	return nil
}

func (m *mockSubject) UnregisterObserver(observer Observer) error {
	delete(m.observers, observer.ObserverID())
	return nil
}

func (m *mockSubject) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	m.events = append(m.events, event)

	for _, registration := range m.observers {
		// Check if observer is interested in this event type
		if len(registration.eventTypes) == 0 {
			// No filter, observer gets all events
			_ = registration.observer.OnEvent(ctx, event)
		} else {
			// Check if event type matches observer's interests
			for _, eventType := range registration.eventTypes {
				if eventType == event.Type() {
					_ = registration.observer.OnEvent(ctx, event)
					break
				}
			}
		}
	}
	return nil
}

func (m *mockSubject) GetObservers() []ObserverInfo {
	info := make([]ObserverInfo, 0, len(m.observers))
	for _, registration := range m.observers {
		info = append(info, ObserverInfo{
			ID:           registration.observer.ObserverID(),
			EventTypes:   registration.eventTypes,
			RegisteredAt: registration.registered,
		})
	}
	return info
}

func TestSubjectObserverInteraction(t *testing.T) {
	t.Parallel()
	subject := newMockSubject()

	// Create observers
	events1 := make([]cloudevents.Event, 0)
	observer1 := NewFunctionalObserver("observer1", func(ctx context.Context, event cloudevents.Event) error {
		events1 = append(events1, event)
		return nil
	})

	events2 := make([]cloudevents.Event, 0)
	observer2 := NewFunctionalObserver("observer2", func(ctx context.Context, event cloudevents.Event) error {
		events2 = append(events2, event)
		return nil
	})

	// Register observers - observer1 gets all events, observer2 only gets "test.specific" events
	err := subject.RegisterObserver(observer1)
	if err != nil {
		t.Fatalf("Failed to register observer1: %v", err)
	}

	err = subject.RegisterObserver(observer2, "test.specific")
	if err != nil {
		t.Fatalf("Failed to register observer2: %v", err)
	}

	// Emit a general event
	generalEvent := NewCloudEvent(
		"test.general",
		"test",
		"general data",
		nil,
	)
	err = subject.NotifyObservers(context.Background(), generalEvent)
	if err != nil {
		t.Fatalf("Failed to notify observers: %v", err)
	}

	// Emit a specific event
	specificEvent := NewCloudEvent(
		"test.specific",
		"test",
		"specific data",
		nil,
	)
	err = subject.NotifyObservers(context.Background(), specificEvent)
	if err != nil {
		t.Fatalf("Failed to notify observers: %v", err)
	}

	// Check observer1 received both events
	if len(events1) != 2 {
		t.Errorf("Expected observer1 to receive 2 events, got %d", len(events1))
	}

	// Check observer2 received only the specific event
	if len(events2) != 1 {
		t.Errorf("Expected observer2 to receive 1 event, got %d", len(events2))
	}
	if len(events2) > 0 && events2[0].Type() != "test.specific" {
		t.Errorf("Expected observer2 to receive 'test.specific' event, got %s", events2[0].Type())
	}

	// Test GetObservers
	observerInfos := subject.GetObservers()
	if len(observerInfos) != 2 {
		t.Errorf("Expected 2 observer infos, got %d", len(observerInfos))
	}

	// Test unregistration
	err = subject.UnregisterObserver(observer1)
	if err != nil {
		t.Fatalf("Failed to unregister observer1: %v", err)
	}

	observerInfos = subject.GetObservers()
	if len(observerInfos) != 1 {
		t.Errorf("Expected 1 observer info after unregistration, got %d", len(observerInfos))
	}
}
