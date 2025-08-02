package modular

import (
	"context"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock types for testing
type mockConfigProvider struct {
	config interface{}
}

func (m *mockConfigProvider) GetConfig() interface{} {
	return m.config
}

func (m *mockConfigProvider) GetDefaultConfig() interface{} {
	return m.config
}

type mockLogger struct {
	entries []mockLogEntry
	mu      sync.Mutex
}

type mockLogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

func (l *mockLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, mockLogEntry{Level: "INFO", Message: msg, Args: args})
}

func (l *mockLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, mockLogEntry{Level: "ERROR", Message: msg, Args: args})
}

func (l *mockLogger) Debug(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, mockLogEntry{Level: "DEBUG", Message: msg, Args: args})
}

func (l *mockLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, mockLogEntry{Level: "WARN", Message: msg, Args: args})
}

type mockModule struct {
	name string
}

func (m *mockModule) Name() string {
	return m.name
}

func (m *mockModule) Init(app Application) error {
	return nil
}

func TestNewCloudEvent(t *testing.T) {
	data := map[string]interface{}{"test": "data"}
	metadata := map[string]interface{}{"key": "value"}

	event := NewCloudEvent("test.event", "test.source", data, metadata)

	assert.Equal(t, "test.event", event.Type())
	assert.Equal(t, "test.source", event.Source())
	assert.Equal(t, cloudevents.VersionV1, event.SpecVersion())
	assert.NotEmpty(t, event.ID())
	assert.False(t, event.Time().IsZero())

	// Check data
	var eventData map[string]interface{}
	err := event.DataAs(&eventData)
	require.NoError(t, err)
	assert.Equal(t, "data", eventData["test"])

	// Check extensions
	extensions := event.Extensions()
	assert.Equal(t, "value", extensions["key"])
}

func TestValidateCloudEvent(t *testing.T) {
	// Valid event
	validEvent := NewCloudEvent("test.event", "test.source", nil, nil)
	err := ValidateCloudEvent(validEvent)
	require.NoError(t, err)

	// Invalid event - missing required fields
	invalidEvent := cloudevents.NewEvent()
	err = ValidateCloudEvent(invalidEvent)
	require.Error(t, err)
}

func TestObservableApplicationCloudEvents(t *testing.T) {
	app := NewObservableApplication(&mockConfigProvider{}, &mockLogger{})

	// Test observer that handles CloudEvents
	cloudEvents := []cloudevents.Event{}
	var mu sync.Mutex

	observer := NewFunctionalObserver("test-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		cloudEvents = append(cloudEvents, event)
		return nil
	})

	// Register observer
	err := app.RegisterObserver(observer)
	require.NoError(t, err)

	// Test NotifyObservers
	testEvent := NewCloudEvent("test.event", "test.source", "test data", nil)
	err = app.NotifyObservers(context.Background(), testEvent)
	require.NoError(t, err)

	// Give time for async notification
	time.Sleep(100 * time.Millisecond)

	// Should have received CloudEvent
	mu.Lock()
	require.Len(t, cloudEvents, 1)
	assert.Equal(t, "test.event", cloudEvents[0].Type())
	assert.Equal(t, "test.source", cloudEvents[0].Source())
	mu.Unlock()
}

func TestObservableApplicationLifecycleCloudEvents(t *testing.T) {
	app := NewObservableApplication(&mockConfigProvider{}, &mockLogger{})

	// Track all events
	allEvents := []cloudevents.Event{}
	var mu sync.Mutex

	observer := NewFunctionalObserver("lifecycle-observer", func(ctx context.Context, event cloudevents.Event) error {
		mu.Lock()
		defer mu.Unlock()
		allEvents = append(allEvents, event)
		return nil
	})

	// Register observer BEFORE registering modules to catch all events
	err := app.RegisterObserver(observer)
	require.NoError(t, err)

	// Test module registration
	module := &mockModule{name: "test-module"}
	app.RegisterModule(module)

	// Test service registration
	err = app.RegisterService("test-service", "test-value")
	require.NoError(t, err)

	// Test application lifecycle
	err = app.Init()
	require.NoError(t, err)

	err = app.Start()
	require.NoError(t, err)

	err = app.Stop()
	require.NoError(t, err)

	// Give time for async events
	time.Sleep(300 * time.Millisecond)

	// Should have received multiple CloudEvents
	mu.Lock()
	assert.GreaterOrEqual(t, len(allEvents), 6) // module, service, init start, init complete, start, stop

	// Check specific events
	eventTypes := make([]string, len(allEvents))
	for i, event := range allEvents {
		eventTypes[i] = event.Type()
		assert.Equal(t, "application", event.Source())
		assert.Equal(t, cloudevents.VersionV1, event.SpecVersion())
		assert.NotEmpty(t, event.ID())
		assert.False(t, event.Time().IsZero())
	}

	assert.Contains(t, eventTypes, EventTypeModuleRegistered)
	assert.Contains(t, eventTypes, EventTypeServiceRegistered)
	assert.Contains(t, eventTypes, EventTypeConfigLoaded)
	assert.Contains(t, eventTypes, EventTypeConfigValidated)
	assert.Contains(t, eventTypes, EventTypeApplicationStarted)
	assert.Contains(t, eventTypes, EventTypeApplicationStopped)
	mu.Unlock()
}
