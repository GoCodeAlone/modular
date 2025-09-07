package modular

import (
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// EventMessage represents an asynchronous message transported via event bus
type EventMessage struct {
	// ID is a unique identifier for this message
	ID string

	// Type indicates the type/category of this event
	Type string

	// Topic is the routing topic for this message
	Topic string

	// Source identifies the origin of this event
	Source string

	// Subject identifies what this event is about
	Subject string

	// Data is the actual event payload
	Data interface{}

	// DataContentType specifies the content type of the data
	DataContentType string

	// Timestamp indicates when this event occurred
	Timestamp time.Time

	// Headers contains additional message headers for routing/metadata
	Headers map[string]string

	// Priority indicates the message priority (higher numbers = higher priority)
	Priority int

	// TTL (Time To Live) indicates when this message expires
	TTL *time.Time

	// RetryCount tracks how many times delivery has been attempted
	RetryCount int

	// MaxRetries specifies the maximum number of delivery attempts
	MaxRetries int

	// CorrelationID links related messages together
	CorrelationID string

	// CausationID references the message that caused this message
	CausationID string

	// CloudEvent is the underlying CloudEvents representation
	CloudEvent *cloudevents.Event

	// Metadata contains additional message-specific metadata
	Metadata map[string]interface{}
}

// EventMessageStatus represents the status of an event message
type EventMessageStatus string

const (
	// EventMessageStatusPending indicates the message is waiting to be sent
	EventMessageStatusPending EventMessageStatus = "pending"

	// EventMessageStatusSent indicates the message has been sent
	EventMessageStatusSent EventMessageStatus = "sent"

	// EventMessageStatusDelivered indicates the message was delivered
	EventMessageStatusDelivered EventMessageStatus = "delivered"

	// EventMessageStatusFailed indicates delivery failed
	EventMessageStatusFailed EventMessageStatus = "failed"

	// EventMessageStatusExpired indicates the message expired
	EventMessageStatusExpired EventMessageStatus = "expired"

	// EventMessageStatusDuplicate indicates this is a duplicate message
	EventMessageStatusDuplicate EventMessageStatus = "duplicate"
)

// EventSubscription represents a subscription to events
type EventSubscription struct {
	// ID is a unique identifier for this subscription
	ID string

	// SubscriberID identifies who created this subscription
	SubscriberID string

	// Topics lists the topics this subscription is interested in
	Topics []string

	// EventTypes lists the event types this subscription is interested in
	EventTypes []string

	// Filters contains additional filtering criteria
	Filters map[string]string

	// Handler is the function called when matching events are received
	Handler EventHandler

	// CreatedAt tracks when this subscription was created
	CreatedAt time.Time

	// LastMessageAt tracks when a message was last received
	LastMessageAt *time.Time

	// MessageCount tracks how many messages have been received
	MessageCount int64

	// Enabled indicates if this subscription is currently active
	Enabled bool

	// DeadLetterTopic specifies where failed messages should go
	DeadLetterTopic string

	// MaxRetries specifies maximum delivery attempts per message
	MaxRetries int

	// AckTimeout specifies how long to wait for message acknowledgment
	AckTimeout time.Duration
}

// EventHandler defines the function signature for handling events
type EventHandler func(message *EventMessage) error

// EventBusStats provides statistics about event bus operations
type EventBusStats struct {
	// TotalMessages is the total number of messages processed
	TotalMessages int64

	// MessagesByTopic breaks down messages by topic
	MessagesByTopic map[string]int64

	// MessagesByType breaks down messages by event type
	MessagesByType map[string]int64

	// ActiveSubscriptions is the number of active subscriptions
	ActiveSubscriptions int

	// FailedDeliveries is the number of failed message deliveries
	FailedDeliveries int64

	// AverageDeliveryTime is the average time to deliver a message
	AverageDeliveryTime time.Duration

	// LastUpdated tracks when these stats were last calculated
	LastUpdated time.Time
}

// EventBusConfiguration represents configuration for the event bus
type EventBusConfiguration struct {
	// BufferSize specifies the size of internal message buffers
	BufferSize int

	// MaxRetries specifies the default maximum retry attempts
	MaxRetries int

	// DeliveryTimeout specifies the timeout for message delivery
	DeliveryTimeout time.Duration

	// EnableDuplicateDetection enables duplicate message detection
	EnableDuplicateDetection bool

	// DuplicateDetectionWindow specifies how long to remember message IDs
	DuplicateDetectionWindow time.Duration

	// EnableMetrics enables collection of event bus metrics
	EnableMetrics bool

	// MetricsInterval specifies how often metrics are calculated
	MetricsInterval time.Duration
}
