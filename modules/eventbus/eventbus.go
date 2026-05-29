package eventbus

import (
	"context"
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v2/event"
)

// EventBus errors
var (
	ErrEventBusNotStarted      = errors.New("event bus not started")
	ErrEventBusShutdownTimeout = errors.New("event bus shutdown timed out")
	ErrEventHandlerNil         = errors.New("event handler cannot be nil")
	ErrInvalidSubscriptionType = errors.New("invalid subscription type")
)

// Event is a CloudEvents SDK event. All events in the eventbus module are
// CloudEvents 1.0 compliant. Use cloudevents.NewEvent() to create one:
//
//	e := cloudevents.NewEvent()
//	e.SetType("user.created")
//	e.SetSource("user-service")
//	e.SetID(uuid.New().String())
//	e.SetData(cloudevents.ApplicationJSON, payload)
type Event = cloudevents.Event

// EventHandler is a function that handles an event.
// Event handlers are called when an event matching their subscription
// topic is published. Handlers should be idempotent when possible and
// handle errors gracefully.
//
// The context can be used for cancellation, timeouts, and passing
// request-scoped values. Handlers should respect context cancellation
// and return promptly when the context is cancelled.
//
// The event's Type() corresponds to the topic, Data() holds the payload,
// and Extensions() contains any additional attributes.
//
// Example handler:
//
//	func userCreatedHandler(ctx context.Context, event Event) error {
//	    var user UserData
//	    if err := event.DataAs(&user); err != nil {
//	        return err
//	    }
//	    return sendWelcomeEmail(ctx, user.Email)
//	}
type EventHandler func(ctx context.Context, event Event) error

// Subscription represents a subscription to a topic.
// Subscriptions are created when a handler is registered for a topic
// and provide methods for managing the subscription lifecycle.
type Subscription interface {
	// Topic returns the topic being subscribed to.
	// This may include wildcard patterns depending on the engine implementation.
	Topic() string

	// ID returns the unique identifier for this subscription.
	// Each subscription gets a unique ID that can be used for tracking,
	// logging, and debugging purposes.
	ID() string

	// IsAsync returns true if this is an asynchronous subscription.
	// Asynchronous subscriptions process events in background workers,
	// while synchronous subscriptions process events immediately.
	IsAsync() bool

	// Cancel cancels the subscription.
	// After calling Cancel, the subscription will no longer receive events.
	// This is equivalent to calling Unsubscribe on the event bus.
	// The method is idempotent and safe to call multiple times.
	Cancel() error
}

// EventBus defines the interface for an event bus implementation.
// This interface abstracts the underlying messaging mechanism, allowing
// the eventbus module to support multiple backends (memory, Redis, Kafka)
// through a common API.
//
// All operations are context-aware to support cancellation and timeouts.
// Implementations should be thread-safe and handle concurrent access properly.
type EventBus interface {
	// Start initializes the event bus.
	// This method is called during module startup and should prepare
	// the event bus for publishing and subscribing operations.
	// For memory buses, this might initialize internal data structures.
	// For network-based buses, this establishes connections.
	Start(ctx context.Context) error

	// Stop shuts down the event bus.
	// This method is called during module shutdown and should cleanup
	// all resources, close connections, and stop background processes.
	//
	// Delivery is at-most-once across teardown: an event already being handled
	// runs to completion, but events still buffered in a subscriber's queue at
	// Stop are NOT delivered — the in-memory engines count them as dropped so
	// the loss is visible via Stats() rather than silent. Stop returns once
	// background goroutines have exited (or the context deadline elapses).
	Stop(ctx context.Context) error

	// Publish sends an event to the specified topic.
	// The event will be delivered to all active subscribers of the topic.
	// The method should handle event queuing, topic routing, and delivery
	// according to the engine's semantics.
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for a topic with synchronous processing.
	// Events matching the topic will be delivered immediately to the handler
	// in the same goroutine that published them. The publisher will wait
	// for the handler to complete before continuing.
	Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error)

	// SubscribeAsync registers a handler for a topic with asynchronous processing.
	// Events matching the topic will be queued for processing by worker goroutines.
	// The publisher can continue immediately without waiting for processing.
	// This is preferred for heavy operations or non-critical event handling.
	SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error)

	// Unsubscribe removes a subscription.
	// After unsubscribing, the subscription will no longer receive events.
	// This method should be idempotent and not return errors for
	// subscriptions that are already cancelled.
	Unsubscribe(ctx context.Context, subscription Subscription) error

	// Topics returns a list of all active topics.
	// This includes only topics that currently have at least one subscriber.
	// Useful for monitoring, debugging, and administrative interfaces.
	Topics() []string

	// SubscriberCount returns the number of subscribers for a topic.
	// This includes both synchronous and asynchronous subscriptions.
	// Returns 0 if the topic has no subscribers or doesn't exist.
	SubscriberCount(topic string) int
}
