package eventbus

// EventBusConfig defines the configuration for the event bus module
type EventBusConfig struct {
	// Engine specifies the event bus engine to use ("memory", "redis", "kafka", etc.)
	Engine string `json:"engine" yaml:"engine" validate:"oneof=memory redis kafka"`

	// MaxEventQueueSize is the maximum number of events to queue per topic
	MaxEventQueueSize int `json:"maxEventQueueSize" yaml:"maxEventQueueSize" validate:"min=1"`

	// DefaultEventBufferSize is the default buffer size for subscription channels
	DefaultEventBufferSize int `json:"defaultEventBufferSize" yaml:"defaultEventBufferSize" validate:"min=1"`

	// WorkerCount is the number of worker goroutines for async event processing
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1"`

	// EventTTL is the time to live for events in seconds
	EventTTL int `json:"eventTTL" yaml:"eventTTL" validate:"min=1"`

	// RetentionDays is how many days to retain event history
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1"`

	// External broker configuration
	ExternalBrokerURL      string `json:"externalBrokerURL" yaml:"externalBrokerURL"`
	ExternalBrokerUser     string `json:"externalBrokerUser" yaml:"externalBrokerUser"`
	ExternalBrokerPassword string `json:"externalBrokerPassword" yaml:"externalBrokerPassword"`
}
