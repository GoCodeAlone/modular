// Package eventbus provides a flexible event-driven messaging system for the modular framework.
//
// This module enables decoupled communication between application components through
// an event bus pattern. It supports both synchronous and asynchronous event processing,
// multiple event bus engines, and configurable event handling strategies.
//
// # Features
//
// The eventbus module offers the following capabilities:
//   - Topic-based event publishing and subscription
//   - Synchronous and asynchronous event processing
//   - Multiple engine support (memory, Redis, Kafka)
//   - Configurable worker pools for async processing
//   - Event metadata and lifecycle tracking
//   - Subscription management with unique identifiers
//   - Event TTL and retention policies
//
// # Configuration
//
// The module can be configured through the EventBusConfig structure:
//
//	config := &EventBusConfig{
//	    Engine:                 "memory",    // or "redis", "kafka"
//	    MaxEventQueueSize:      1000,        // events per topic queue
//	    DefaultEventBufferSize: 10,          // subscription channel buffer
//	    WorkerCount:            5,           // async processing workers
//	    EventTTL:               3600,        // event time-to-live in seconds
//	    RetentionDays:          7,           // event history retention
//	    ExternalBrokerURL:      "",          // for external brokers
//	    ExternalBrokerUser:     "",          // broker authentication
//	    ExternalBrokerPassword: "",          // broker password
//	}
//
// # Service Registration
//
// The module registers itself as a service for dependency injection:
//
//	// Get the event bus service
//	eventBus := app.GetService("eventbus.provider").(*EventBusModule)
//
//	// Publish an event
//	err := eventBus.Publish(ctx, "user.created", userData)
//
//	// Subscribe to events
//	subscription, err := eventBus.Subscribe(ctx, "user.*", userEventHandler)
//
// # Usage Examples
//
// Basic event publishing:
//
//	// Publish a simple event
//	err := eventBus.Publish(ctx, "order.placed", orderData)
//
//	// Publish with custom metadata
//	event := Event{
//	    Topic:   "payment.processed",
//	    Payload: paymentData,
//	    Metadata: map[string]interface{}{
//	        "source": "payment-service",
//	        "version": "1.2.0",
//	    },
//	}
//	err := eventBus.Publish(ctx, event.Topic, event.Payload)
//
// Event subscription patterns:
//
//	// Synchronous subscription
//	subscription, err := eventBus.Subscribe(ctx, "user.updated", func(ctx context.Context, event Event) error {
//	    user := event.Payload.(UserData)
//	    return updateUserCache(user)
//	})
//
//	// Asynchronous subscription for heavy processing
//	asyncSub, err := eventBus.SubscribeAsync(ctx, "image.uploaded", func(ctx context.Context, event Event) error {
//	    imageData := event.Payload.(ImageData)
//	    return processImageThumbnails(imageData)
//	})
//
//	// Wildcard subscriptions
//	allOrdersSub, err := eventBus.Subscribe(ctx, "order.*", orderEventHandler)
//
// Subscription management:
//
//	// Check subscription details
//	fmt.Printf("Subscribed to: %s (ID: %s, Async: %v)",
//	    subscription.Topic(), subscription.ID(), subscription.IsAsync())
//
//	// Cancel specific subscriptions
//	err := eventBus.Unsubscribe(ctx, subscription)
//
//	// Or cancel through the subscription itself
//	err := subscription.Cancel()
//
// # Event Processing Patterns
//
// The module supports different event processing patterns:
//
// **Synchronous Processing**: Events are processed immediately in the same goroutine
// that published them. Best for lightweight operations and when ordering is important.
//
// **Asynchronous Processing**: Events are queued and processed by worker goroutines.
// Best for heavy operations, external API calls, or when you don't want to block
// the publisher.
//
// # Engine Support
//
// Currently supported engines:
//   - **memory**: In-process event bus using Go channels
//   - **redis**: Distributed event bus using Redis pub/sub (planned)
//   - **kafka**: Enterprise event bus using Apache Kafka (planned)
package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/CrisisTextLine/modular"
)

// ModuleName is the unique identifier for the eventbus module.
const ModuleName = "eventbus"

// ServiceName is the name of the service provided by this module.
// Other modules can use this name to request the event bus service through dependency injection.
const ServiceName = "eventbus.provider"

// EventBusModule provides event-driven messaging capabilities for the modular framework.
// It implements a publish-subscribe pattern with support for multiple event bus engines,
// asynchronous processing, and flexible subscription management.
//
// The module implements the following interfaces:
//   - modular.Module: Basic module lifecycle
//   - modular.Configurable: Configuration management
//   - modular.ServiceAware: Service dependency management
//   - modular.Startable: Startup logic
//   - modular.Stoppable: Shutdown logic
//   - EventBus: Event publishing and subscription interface
//
// Event processing is thread-safe and supports concurrent publishers and subscribers.
type EventBusModule struct {
	name      string
	config    *EventBusConfig
	logger    modular.Logger
	eventbus  EventBus
	mutex     sync.RWMutex
	isStarted bool
}

// NewModule creates a new instance of the event bus module.
// This is the primary constructor for the eventbus module and should be used
// when registering the module with the application.
//
// Example:
//
//	app.RegisterModule(eventbus.NewModule())
func NewModule() modular.Module {
	return &EventBusModule{
		name: ModuleName,
	}
}

// Name returns the unique identifier for this module.
// This name is used for service registration, dependency resolution,
// and configuration section identification.
func (m *EventBusModule) Name() string {
	return m.name
}

// RegisterConfig registers the module's configuration structure.
// This method is called during application initialization to register
// the default configuration values for the eventbus module.
//
// Default configuration:
//   - Engine: "memory"
//   - MaxEventQueueSize: 1000 events per topic
//   - DefaultEventBufferSize: 10 events per subscription channel
//   - WorkerCount: 5 async processing workers
//   - EventTTL: 3600 seconds (1 hour)
//   - RetentionDays: 7 days for event history
//   - ExternalBroker settings: empty (not used for memory engine)
func (m *EventBusModule) RegisterConfig(app modular.Application) error {
	// Register the configuration with default values
	defaultConfig := &EventBusConfig{
		Engine:                 "memory",
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		WorkerCount:            5,
		EventTTL:               3600,
		RetentionDays:          7,
		ExternalBrokerURL:      "",
		ExternalBrokerUser:     "",
		ExternalBrokerPassword: "",
	}

	app.RegisterConfigSection(m.Name(), modular.NewStdConfigProvider(defaultConfig))
	return nil
}

// Init initializes the eventbus module with the application context.
// This method is called after all modules have been registered and their
// configurations loaded. It sets up the event bus engine based on configuration.
//
// The initialization process:
//  1. Retrieves the module's configuration
//  2. Sets up logging
//  3. Initializes the appropriate event bus engine
//  4. Prepares the event bus for startup
//
// Supported engines:
//   - "memory": In-process event bus using Go channels
//   - fallback: defaults to memory engine for unknown engines
func (m *EventBusModule) Init(app modular.Application) error {
	// Retrieve the registered config section for access
	cfg, err := app.GetConfigSection(m.name)
	if err != nil {
		return fmt.Errorf("failed to get config section '%s': %w", m.name, err)
	}

	m.config = cfg.GetConfig().(*EventBusConfig)
	m.logger = app.Logger()

	// Initialize the event bus based on configuration
	switch m.config.Engine {
	case "memory":
		m.eventbus = NewMemoryEventBus(m.config)
		m.logger.Info("Using memory event bus")
	default:
		m.eventbus = NewMemoryEventBus(m.config)
		m.logger.Warn("Unknown event bus engine specified, using memory engine", "specified", m.config.Engine)
	}

	m.logger.Info("Event bus module initialized")
	return nil
}

// Start performs startup logic for the module.
// This method starts the event bus engine and begins processing events.
// It's called after all modules have been initialized and are ready to start.
//
// The startup process:
//  1. Checks if already started (idempotent)
//  2. Starts the underlying event bus engine
//  3. Initializes worker pools for async processing
//  4. Prepares topic management and subscription tracking
//
// This method is thread-safe and can be called multiple times safely.
func (m *EventBusModule) Start(ctx context.Context) error {
	m.logger.Info("Starting event bus module")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isStarted {
		return nil
	}

	// Start the event bus
	err := m.eventbus.Start(ctx)
	if err != nil {
		return fmt.Errorf("starting event bus: %w", err)
	}

	m.isStarted = true
	m.logger.Info("Event bus started")
	return nil
}

// Stop performs shutdown logic for the module.
// This method gracefully shuts down the event bus, ensuring all in-flight
// events are processed and all subscriptions are properly cleaned up.
//
// The shutdown process:
//  1. Checks if already stopped (idempotent)
//  2. Stops accepting new events
//  3. Waits for in-flight events to complete
//  4. Cancels all active subscriptions
//  5. Shuts down worker pools
//  6. Closes the underlying event bus engine
//
// This method is thread-safe and can be called multiple times safely.
func (m *EventBusModule) Stop(ctx context.Context) error {
	m.logger.Info("Stopping event bus module")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isStarted {
		return nil
	}

	// Stop the event bus
	err := m.eventbus.Stop(ctx)
	if err != nil {
		return fmt.Errorf("stopping event bus: %w", err)
	}

	m.isStarted = false
	m.logger.Info("Event bus stopped")
	return nil
}

// Dependencies returns the names of modules this module depends on.
// The eventbus module operates independently and has no dependencies.
func (m *EventBusModule) Dependencies() []string {
	return nil
}

// ProvidesServices declares services provided by this module.
// The eventbus module provides an event bus service that can be injected
// into other modules for event-driven communication.
//
// Provided services:
//   - "eventbus.provider": The main event bus service interface
func (m *EventBusModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:        ServiceName,
			Description: "Event bus for message distribution",
			Instance:    m,
		},
	}
}

// RequiresServices declares services required by this module.
// The eventbus module operates independently and requires no external services.
func (m *EventBusModule) RequiresServices() []modular.ServiceDependency {
	return nil
}

// Constructor provides a dependency injection constructor for the module.
// This method is used by the dependency injection system to create
// the module instance with any required services.
func (m *EventBusModule) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		return m, nil
	}
}

// Publish publishes an event to the event bus.
// Creates an Event struct with the provided topic and payload, then
// sends it through the event bus for processing by subscribers.
//
// The event will be delivered to all active subscribers of the topic.
// Topic patterns and wildcards may be supported depending on the engine.
//
// Example:
//
//	err := eventBus.Publish(ctx, "user.created", userData)
//	err := eventBus.Publish(ctx, "order.payment.failed", paymentData)
func (m *EventBusModule) Publish(ctx context.Context, topic string, payload interface{}) error {
	event := Event{
		Topic:   topic,
		Payload: payload,
	}
	err := m.eventbus.Publish(ctx, event)
	if err != nil {
		return fmt.Errorf("publishing event to topic %s: %w", topic, err)
	}
	return nil
}

// Subscribe subscribes to a topic on the event bus with synchronous processing.
// The provided handler will be called immediately when an event is published
// to the specified topic. The handler blocks the event delivery until it completes.
//
// Use synchronous subscriptions for:
//   - Lightweight event processing
//   - When event ordering is important
//   - Critical event handlers that must complete before continuing
//
// Example:
//
//	subscription, err := eventBus.Subscribe(ctx, "user.login", func(ctx context.Context, event Event) error {
//	    user := event.Payload.(UserData)
//	    return updateLastLoginTime(user.ID)
//	})
func (m *EventBusModule) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	sub, err := m.eventbus.Subscribe(ctx, topic, handler)
	if err != nil {
		return nil, fmt.Errorf("subscribing to topic %s: %w", topic, err)
	}
	return sub, nil
}

// SubscribeAsync subscribes to a topic with asynchronous event processing.
// The provided handler will be queued for processing by worker goroutines,
// allowing the event publisher to continue without waiting for processing.
//
// Use asynchronous subscriptions for:
//   - Heavy processing operations
//   - External API calls
//   - Non-critical event handlers
//   - When you want to avoid blocking publishers
//
// Example:
//
//	subscription, err := eventBus.SubscribeAsync(ctx, "image.uploaded", func(ctx context.Context, event Event) error {
//	    imageData := event.Payload.(ImageData)
//	    return generateThumbnails(imageData)
//	})
func (m *EventBusModule) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	sub, err := m.eventbus.SubscribeAsync(ctx, topic, handler)
	if err != nil {
		return nil, fmt.Errorf("subscribing async to topic %s: %w", topic, err)
	}
	return sub, nil
}

// Unsubscribe cancels a subscription and stops receiving events.
// The subscription will be removed from the event bus and no longer
// receive events for its topic.
//
// This method is idempotent - calling it multiple times on the same
// subscription is safe and will not cause errors.
//
// Example:
//
//	err := eventBus.Unsubscribe(ctx, subscription)
func (m *EventBusModule) Unsubscribe(ctx context.Context, subscription Subscription) error {
	err := m.eventbus.Unsubscribe(ctx, subscription)
	if err != nil {
		return fmt.Errorf("unsubscribing: %w", err)
	}
	return nil
}

// Topics returns a list of all active topics that have subscribers.
// This can be useful for debugging, monitoring, or building administrative
// interfaces that show current event bus activity.
//
// Example:
//
//	activeTopics := eventBus.Topics()
//	for _, topic := range activeTopics {
//	    count := eventBus.SubscriberCount(topic)
//	    fmt.Printf("Topic: %s, Subscribers: %d\n", topic, count)
//	}
func (m *EventBusModule) Topics() []string {
	return m.eventbus.Topics()
}

// SubscriberCount returns the number of active subscribers for a topic.
// This includes both synchronous and asynchronous subscriptions.
// Returns 0 if the topic has no subscribers.
//
// Example:
//
//	count := eventBus.SubscriberCount("user.created")
//	if count == 0 {
//	    log.Warn("No subscribers for user creation events")
//	}
func (m *EventBusModule) SubscriberCount(topic string) int {
	return m.eventbus.SubscriberCount(topic)
}
