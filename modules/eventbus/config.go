package eventbus

// EventBusConfig defines the configuration for the event bus module
type EventBusConfig struct {
	// Engine specifies the event bus engine to use ("memory", "redis", "kafka", etc.)
	Engine string `json:"engine" yaml:"engine" validate:"oneof=memory redis kafka" env:"ENGINE"`

	// MaxEventQueueSize is the maximum number of events to queue per topic
	MaxEventQueueSize int `json:"maxEventQueueSize" yaml:"maxEventQueueSize" validate:"min=1" env:"MAX_EVENT_QUEUE_SIZE"`

	// DefaultEventBufferSize is the default buffer size for subscription channels
	DefaultEventBufferSize int `json:"defaultEventBufferSize" yaml:"defaultEventBufferSize" validate:"min=1" env:"DEFAULT_EVENT_BUFFER_SIZE"`

	// WorkerCount is the number of worker goroutines for async event processing
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1" env:"WORKER_COUNT"`

	// EventTTL is the time to live for events in seconds
	EventTTL int `json:"eventTTL" yaml:"eventTTL" validate:"min=1" env:"EVENT_TTL"`

	// RetentionDays is how many days to retain event history
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1" env:"RETENTION_DAYS"`

	// External broker configuration
	ExternalBrokerURL      string `json:"externalBrokerURL" yaml:"externalBrokerURL" env:"EXTERNAL_BROKER_URL"`
	ExternalBrokerUser     string `json:"externalBrokerUser" yaml:"externalBrokerUser" env:"EXTERNAL_BROKER_USER"`
	ExternalBrokerPassword string `json:"externalBrokerPassword" yaml:"externalBrokerPassword" env:"EXTERNAL_BROKER_PASSWORD"`
}
