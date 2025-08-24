package eventbus

// Event type constants for eventbus module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Message events
	EventTypeMessagePublished = "com.modular.eventbus.message.published"
	EventTypeMessageReceived  = "com.modular.eventbus.message.received"
	EventTypeMessageFailed    = "com.modular.eventbus.message.failed"

	// Topic events
	EventTypeTopicCreated = "com.modular.eventbus.topic.created"
	EventTypeTopicDeleted = "com.modular.eventbus.topic.deleted"

	// Subscription events
	EventTypeSubscriptionCreated = "com.modular.eventbus.subscription.created"
	EventTypeSubscriptionRemoved = "com.modular.eventbus.subscription.removed"

	// Bus lifecycle events
	EventTypeBusStarted = "com.modular.eventbus.bus.started"
	EventTypeBusStopped = "com.modular.eventbus.bus.stopped"

	// Configuration events
	EventTypeConfigLoaded = "com.modular.eventbus.config.loaded"
)
