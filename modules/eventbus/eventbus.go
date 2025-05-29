package eventbus

import (
	"context"
	"time"
)

// Event represents a message in the event bus
type Event struct {
	// Topic is the channel or subject of the event
	Topic string `json:"topic"`

	// Payload is the data associated with the event
	Payload interface{} `json:"payload"`

	// Metadata contains additional information about the event
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when the event was created
	CreatedAt time.Time `json:"createdAt"`

	// ProcessingStarted is when the event processing started
	ProcessingStarted *time.Time `json:"processingStarted,omitempty"`

	// ProcessingCompleted is when the event processing completed
	ProcessingCompleted *time.Time `json:"processingCompleted,omitempty"`
}

// EventHandler is a function that handles an event
type EventHandler func(ctx context.Context, event Event) error

// Subscription represents a subscription to a topic
type Subscription interface {
	// Topic returns the topic being subscribed to
	Topic() string

	// ID returns the unique identifier for this subscription
	ID() string

	// IsAsync returns true if this is an asynchronous subscription
	IsAsync() bool

	// Cancel cancels the subscription
	Cancel() error
}

// EventBus defines the interface for an event bus implementation
type EventBus interface {
	// Start initializes the event bus
	Start(ctx context.Context) error

	// Stop shuts down the event bus
	Stop(ctx context.Context) error

	// Publish sends an event to the specified topic
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for a topic with synchronous processing
	Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error)

	// SubscribeAsync registers a handler for a topic with asynchronous processing
	SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error)

	// Unsubscribe removes a subscription
	Unsubscribe(ctx context.Context, subscription Subscription) error

	// Topics returns a list of all active topics
	Topics() []string

	// SubscriberCount returns the number of subscribers for a topic
	SubscriberCount(topic string) int
}
