package eventbus

import (
	"context"
	"sync"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cevent "github.com/cloudevents/sdk-go/v2/event"
	"github.com/google/uuid"
)

// newTestCloudEvent creates a CloudEvents event for testing. It sets the type
// (used as the topic), source, a random ID, and optionally marshals data into
// the event payload.
func newTestCloudEvent(eventType string, data interface{}) cevent.Event {
	e := cevent.New()
	e.SetType(eventType)
	e.SetSource("test")
	e.SetID(uuid.New().String())
	if data != nil {
		_ = e.SetData("application/json", data)
	}
	return e
}

// ==============================================================================
// TEST CONTEXT AND SETUP
// ==============================================================================
// This file defines the test context structure, test observer, and basic
// context management functions.

// EventBus BDD Test Context
type EventBusBDDTestContext struct {
	app               modular.Application
	module            *EventBusModule
	service           *EventBusModule
	eventbusConfig    *EventBusConfig
	lastError         error
	receivedEvents    []Event
	eventHandlers     map[string]func(context.Context, Event) error
	subscriptions     map[string]Subscription
	lastSubscription  Subscription
	asyncProcessed    bool
	publishingBlocked bool
	handlerErrors     []error
	activeTopics      []string
	subscriberCounts  map[string]int
	mutex             sync.Mutex
	// Event observation
	eventObserver *testEventObserver
	// Multi-engine fields
	customEngineType     string
	publishedTopics      map[string]bool
	totalSubscriberCount int
	// Tenant testing fields
	tenantEventHandlers  map[string]map[string]func(context.Context, Event) error // tenant -> topic -> handler
	tenantReceivedEvents map[string][]Event                                       // tenant -> events received
	tenantSubscriptions  map[string]map[string]Subscription                       // tenant -> topic -> subscription
	tenantEngineConfig   map[string]string                                        // tenant -> engine type
	errorTopic           string                                                   // topic that caused an error for testing
}

// Test event observer for capturing emitted events
type testEventObserver struct {
	events []cloudevents.Event
	mutex  sync.Mutex
}

func newTestEventObserver() *testEventObserver {
	return &testEventObserver{
		events: make([]cloudevents.Event, 0),
	}
}

func (t *testEventObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.events = append(t.events, event.Clone())
	return nil
}

func (t *testEventObserver) ObserverID() string {
	return "test-observer-eventbus"
}

func (t *testEventObserver) GetEvents() []cloudevents.Event {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	events := make([]cloudevents.Event, len(t.events))
	copy(events, t.events)
	return events
}

func (t *testEventObserver) ClearEvents() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.events = make([]cloudevents.Event, 0)
}

func (ctx *EventBusBDDTestContext) resetContext() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	ctx.app = nil
	ctx.module = nil
	ctx.service = nil
	ctx.eventbusConfig = nil
	ctx.lastError = nil
	ctx.receivedEvents = nil
	ctx.eventHandlers = make(map[string]func(context.Context, Event) error)
	ctx.subscriptions = make(map[string]Subscription)
	ctx.lastSubscription = nil
	ctx.asyncProcessed = false
	ctx.publishingBlocked = false
	ctx.handlerErrors = nil
	ctx.activeTopics = nil
	ctx.subscriberCounts = make(map[string]int)
	ctx.eventObserver = nil
	// Initialize tenant-specific maps
	ctx.tenantEventHandlers = make(map[string]map[string]func(context.Context, Event) error)
	ctx.tenantReceivedEvents = make(map[string][]Event)
	ctx.tenantSubscriptions = make(map[string]map[string]Subscription)
	ctx.tenantEngineConfig = make(map[string]string)
}
