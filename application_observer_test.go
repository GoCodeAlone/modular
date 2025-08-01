package modular

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

var errObserver = errors.New("observer error")

func TestObservableApplication_RegisterObserver(t *testing.T) {
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), &TestObserverLogger{})

	// Create a test observer
	events := make([]cloudevents.Event, 0)
	var mu sync.Mutex
	observer := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
		return nil
	})

	// Register observer for specific event types
	err := app.RegisterObserver(observer, EventTypeModuleRegistered, EventTypeServiceRegistered)
	if err != nil {
		t.Fatalf("Failed to register observer: %v", err)
	}

	// Check observer info
	observerInfos := app.GetObservers()
	if len(observerInfos) != 1 {
		t.Errorf("Expected 1 observer, got %d", len(observerInfos))
	}

	if observerInfos[0].ID != "test-observer" {
		t.Errorf("Expected observer ID 'test-observer', got %s", observerInfos[0].ID)
	}

	// Check event types
	expectedEventTypes := map[string]bool{
		EventTypeModuleRegistered:  true,
		EventTypeServiceRegistered: true,
	}
	for _, eventType := range observerInfos[0].EventTypes {
		if !expectedEventTypes[eventType] {
			t.Errorf("Unexpected event type: %s", eventType)
		}
		delete(expectedEventTypes, eventType)
	}
	if len(expectedEventTypes) > 0 {
		t.Errorf("Missing event types: %v", expectedEventTypes)
	}
}

func TestObservableApplication_UnregisterObserver(t *testing.T) {
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), &TestObserverLogger{})

	observer := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		return nil
	})

	// Register and then unregister
	err := app.RegisterObserver(observer)
	if err != nil {
		t.Fatalf("Failed to register observer: %v", err)
	}

	observerInfos := app.GetObservers()
	if len(observerInfos) != 1 {
		t.Errorf("Expected 1 observer after registration, got %d", len(observerInfos))
	}

	err = app.UnregisterObserver(observer)
	if err != nil {
		t.Fatalf("Failed to unregister observer: %v", err)
	}

	observerInfos = app.GetObservers()
	if len(observerInfos) != 0 {
		t.Errorf("Expected 0 observers after unregistration, got %d", len(observerInfos))
	}

	// Test idempotent unregistration
	err = app.UnregisterObserver(observer)
	if err != nil {
		t.Errorf("Unregistering non-existent observer should not error: %v", err)
	}
}

func TestObservableApplication_NotifyObservers(t *testing.T) {
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), &TestObserverLogger{})

	// Create observers with different event type filters
	events1 := make([]cloudevents.Event, 0)
	var mu1 sync.Mutex
	observer1 := NewFunctionalObserver("observer1", func(ctx context.Context, event cloudevents.Event) error {
		mu1.Lock()
		defer mu1.Unlock()
		events1 = append(events1, event)
		return nil
	})

	events2 := make([]cloudevents.Event, 0)
	var mu2 sync.Mutex
	observer2 := NewFunctionalObserver("observer2", func(ctx context.Context, event cloudevents.Event) error {
		mu2.Lock()
		defer mu2.Unlock()
		events2 = append(events2, event)
		return nil
	})

	// Register observers - observer1 gets all events, observer2 only gets module events
	err := app.RegisterObserver(observer1)
	if err != nil {
		t.Fatalf("Failed to register observer1: %v", err)
	}

	err = app.RegisterObserver(observer2, EventTypeModuleRegistered)
	if err != nil {
		t.Fatalf("Failed to register observer2: %v", err)
	}

	// Emit different types of events
	moduleEvent := NewCloudEvent(
		EventTypeModuleRegistered,
		"test",
		"module data",
		nil,
	)

	serviceEvent := NewCloudEvent(
		EventTypeServiceRegistered,
		"test",
		"service data",
		nil,
	)

	err = app.NotifyObservers(context.Background(), moduleEvent)
	if err != nil {
		t.Fatalf("Failed to notify observers: %v", err)
	}

	err = app.NotifyObservers(context.Background(), serviceEvent)
	if err != nil {
		t.Fatalf("Failed to notify observers: %v", err)
	}

	// Wait a bit for async notifications
	time.Sleep(100 * time.Millisecond)

	// Check observer1 received both events
	mu1.Lock()
	if len(events1) != 2 {
		t.Errorf("Expected observer1 to receive 2 events, got %d", len(events1))
	}
	mu1.Unlock()

	// Check observer2 received only the module event
	mu2.Lock()
	if len(events2) != 1 {
		t.Errorf("Expected observer2 to receive 1 event, got %d", len(events2))
	}
	if len(events2) > 0 && events2[0].Type() != EventTypeModuleRegistered {
		t.Errorf("Expected observer2 to receive module event, got %s", events2[0].Type())
	}
	mu2.Unlock()
}

func TestObservableApplication_ModuleRegistrationEvents(t *testing.T) {
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), &TestObserverLogger{})

	// Register observer for module events
	events := make([]cloudevents.Event, 0)
	var mu sync.Mutex
	observer := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
		return nil
	})

	err := app.RegisterObserver(observer, EventTypeModuleRegistered)
	if err != nil {
		t.Fatalf("Failed to register observer: %v", err)
	}

	// Register a test module
	testModule := &TestObserverModule{name: "test-module"}
	app.RegisterModule(testModule)

	// Wait for async event
	time.Sleep(100 * time.Millisecond)

	// Check event was emitted
	mu.Lock()
	if len(events) != 1 {
		t.Errorf("Expected 1 module registration event, got %d", len(events))
	}

	if len(events) > 0 {
		event := events[0]
		if event.Type() != EventTypeModuleRegistered {
			t.Errorf("Expected event type %s, got %s", EventTypeModuleRegistered, event.Type())
		}
		if event.Source() != "application" {
			t.Errorf("Expected event source 'application', got %s", event.Source())
		}
	}
	mu.Unlock()
}

func TestObservableApplication_ServiceRegistrationEvents(t *testing.T) {
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), &TestObserverLogger{})

	// Register observer for service events
	events := make([]cloudevents.Event, 0)
	var mu sync.Mutex
	observer := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
		return nil
	})

	err := app.RegisterObserver(observer, EventTypeServiceRegistered)
	if err != nil {
		t.Fatalf("Failed to register observer: %v", err)
	}

	// Register a test service
	testService := &TestObserverStorage{}
	err = app.RegisterService("test-service", testService)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Wait for async event
	time.Sleep(100 * time.Millisecond)

	// Check event was emitted
	mu.Lock()
	if len(events) != 1 {
		t.Errorf("Expected 1 service registration event, got %d", len(events))
	}

	if len(events) > 0 {
		event := events[0]
		if event.Type() != EventTypeServiceRegistered {
			t.Errorf("Expected event type %s, got %s", EventTypeServiceRegistered, event.Type())
		}
		if event.Source() != "application" {
			t.Errorf("Expected event source 'application', got %s", event.Source())
		}
	}
	mu.Unlock()
}

// Test observer error handling
func TestObservableApplication_ObserverErrorHandling(t *testing.T) {
	logger := &TestObserverLogger{}
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), logger)

	// Create an observer that always errors
	errorObserver := NewFunctionalObserver("error-observer", func(ctx context.Context, event cloudevents.Event) error {
		return errObserver
	})

	// Create a normal observer
	events := make([]cloudevents.Event, 0)
	var mu sync.Mutex
	normalObserver := NewFunctionalObserver("normal-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
		return nil
	})

	// Register both observers
	err := app.RegisterObserver(errorObserver)
	if err != nil {
		t.Fatalf("Failed to register error observer: %v", err)
	}

	err = app.RegisterObserver(normalObserver)
	if err != nil {
		t.Fatalf("Failed to register normal observer: %v", err)
	}

	// Emit an event
	testEvent := NewCloudEvent(
		"test.event",
		"test",
		"test data",
		nil,
	)

	err = app.NotifyObservers(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("NotifyObservers should not return error even if observers fail: %v", err)
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Normal observer should still receive the event despite error observer failing
	mu.Lock()
	if len(events) != 1 {
		t.Errorf("Expected normal observer to receive 1 event despite error observer, got %d", len(events))
	}
	mu.Unlock()
}

// Mock types for testing - using unique names to avoid conflicts
type TestObserverModule struct {
	name string
}

func (m *TestObserverModule) Name() string               { return m.name }
func (m *TestObserverModule) Init(app Application) error { return nil }

type TestObserverLogger struct {
	entries []LogEntry
	mu      sync.Mutex
}

type LogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

func (l *TestObserverLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, LogEntry{Level: "INFO", Message: msg, Args: args})
}

func (l *TestObserverLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, LogEntry{Level: "ERROR", Message: msg, Args: args})
}

func (l *TestObserverLogger) Debug(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, LogEntry{Level: "DEBUG", Message: msg, Args: args})
}

func (l *TestObserverLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, LogEntry{Level: "WARN", Message: msg, Args: args})
}

type TestObserverStorage struct{}
