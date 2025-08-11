package eventbus

import (
	"time"
)

// EventBusConfig defines the configuration for the event bus module.
// This structure contains all the settings needed to configure event processing,
// worker pools, event retention, and external broker connections.
//
// Configuration can be provided through JSON, YAML, or environment variables.
// The struct tags define the mapping for each configuration source and
// validation rules.
//
// Example YAML configuration:
//
//	engine: "memory"
//	maxEventQueueSize: 2000
//	defaultEventBufferSize: 20
//	workerCount: 10
//	eventTTL: 7200
//	retentionDays: 14
//	externalBrokerURL: "redis://localhost:6379"
//	externalBrokerUser: "eventbus_user"
//	externalBrokerPassword: "secure_password"
//
// Example environment variables:
//
//	EVENTBUS_ENGINE=memory
//	EVENTBUS_MAX_EVENT_QUEUE_SIZE=1000
//	EVENTBUS_WORKER_COUNT=5
type EventBusConfig struct {
	// Engine specifies the event bus engine to use.
	// Supported values: "memory", "redis", "kafka"
	// Default: "memory"
	Engine string `json:"engine" yaml:"engine" validate:"oneof=memory redis kafka" env:"ENGINE"`

	// MaxEventQueueSize is the maximum number of events to queue per topic.
	// When this limit is reached, new events may be dropped or publishers
	// may be blocked, depending on the engine implementation.
	// Must be at least 1.
	MaxEventQueueSize int `json:"maxEventQueueSize" yaml:"maxEventQueueSize" validate:"min=1" env:"MAX_EVENT_QUEUE_SIZE"`

	// DefaultEventBufferSize is the default buffer size for subscription channels.
	// This affects how many events can be buffered for each subscription before
	// blocking. Larger buffers can improve performance but use more memory.
	// Must be at least 1.
	DefaultEventBufferSize int `json:"defaultEventBufferSize" yaml:"defaultEventBufferSize" validate:"min=1" env:"DEFAULT_EVENT_BUFFER_SIZE"`

	// WorkerCount is the number of worker goroutines for async event processing.
	// These workers process events from asynchronous subscriptions. More workers
	// can increase throughput but also increase resource usage.
	// Must be at least 1.
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1" env:"WORKER_COUNT"`

	// EventTTL is the time to live for events.
	// Events older than this value may be automatically removed from queues
	// or marked as expired. Used for event cleanup and storage management.
	EventTTL time.Duration `json:"eventTTL" yaml:"eventTTL" env:"EVENT_TTL" default:"3600s"`

	// RetentionDays is how many days to retain event history.
	// This affects event storage and cleanup policies. Longer retention
	// allows for event replay and debugging but requires more storage.
	// Must be at least 1.
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1" env:"RETENTION_DAYS"`

	// ExternalBrokerURL is the connection URL for external message brokers.
	// Used when the engine is set to "redis" or "kafka". The format depends
	// on the specific broker type.
	// Examples:
	//   Redis: "redis://localhost:6379" or "redis://user:pass@host:port/db"
	//   Kafka: "kafka://localhost:9092" or "kafka://broker1:9092,broker2:9092"
	ExternalBrokerURL string `json:"externalBrokerURL" yaml:"externalBrokerURL" env:"EXTERNAL_BROKER_URL"`

	// ExternalBrokerUser is the username for external broker authentication.
	// Used when the external broker requires authentication.
	// Leave empty if the broker doesn't require authentication.
	ExternalBrokerUser string `json:"externalBrokerUser" yaml:"externalBrokerUser" env:"EXTERNAL_BROKER_USER"`

	// ExternalBrokerPassword is the password for external broker authentication.
	// Used when the external broker requires authentication.
	// Leave empty if the broker doesn't require authentication.
	// This should be kept secure and may be provided via environment variables.
	ExternalBrokerPassword string `json:"externalBrokerPassword" yaml:"externalBrokerPassword" env:"EXTERNAL_BROKER_PASSWORD"`
}
