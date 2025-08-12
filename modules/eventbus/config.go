package eventbus

import (
	"fmt"
	"time"
)

// EngineConfig defines the configuration for an individual event bus engine.
// Each engine can have its own specific configuration requirements.
type EngineConfig struct {
	// Name is the unique identifier for this engine instance.
	// Used for routing and engine selection.
	Name string `json:"name" yaml:"name" validate:"required"`

	// Type specifies the engine implementation to use.
	// Supported values: "memory", "redis", "kafka", "kinesis", "custom"
	Type string `json:"type" yaml:"type" validate:"required,oneof=memory redis kafka kinesis custom"`

	// Config contains engine-specific configuration as a map.
	// The structure depends on the engine type.
	Config map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// RoutingRule defines how topics are routed to engines.
type RoutingRule struct {
	// Topics is a list of topic patterns to match.
	// Supports wildcards like "user.*" or exact matches.
	Topics []string `json:"topics" yaml:"topics" validate:"required,min=1"`

	// Engine is the name of the engine to route matching topics to.
	// Must match the name of a configured engine.
	Engine string `json:"engine" yaml:"engine" validate:"required"`
}

// EventBusConfig defines the configuration for the event bus module.
// This structure supports both single-engine (legacy) and multi-engine configurations.
//
// Example single-engine YAML configuration (legacy, still supported):
//
//	engine: "memory"
//	maxEventQueueSize: 1000
//	workerCount: 5
//
// Example multi-engine YAML configuration:
//
//	engines:
//	  - name: "memory"
//	    type: "memory"
//	    config:
//	      workerCount: 5
//	      maxEventQueueSize: 1000
//	  - name: "redis"
//	    type: "redis"
//	    config:
//	      url: "redis://localhost:6379"
//	      db: 0
//	routing:
//	  - topics: ["user.*", "auth.*"]
//	    engine: "memory"
//	  - topics: ["*"]
//	    engine: "redis"
type EventBusConfig struct {
	// --- Single Engine Configuration (Legacy Support) ---
	
	// Engine specifies the event bus engine to use for single-engine mode.
	// Supported values: "memory", "redis", "kafka", "kinesis"
	// Default: "memory"
	// Note: This field is used only when Engines is empty (legacy mode)
	Engine string `json:"engine,omitempty" yaml:"engine,omitempty" validate:"omitempty,oneof=memory redis kafka kinesis" env:"ENGINE"`

	// MaxEventQueueSize is the maximum number of events to queue per topic.
	// When this limit is reached, new events may be dropped or publishers
	// may be blocked, depending on the engine implementation.
	// Must be at least 1. Used in single-engine mode.
	MaxEventQueueSize int `json:"maxEventQueueSize,omitempty" yaml:"maxEventQueueSize,omitempty" validate:"omitempty,min=1" env:"MAX_EVENT_QUEUE_SIZE"`

	// DefaultEventBufferSize is the default buffer size for subscription channels.
	// This affects how many events can be buffered for each subscription before
	// blocking. Larger buffers can improve performance but use more memory.
	// Must be at least 1. Used in single-engine mode.
	DefaultEventBufferSize int `json:"defaultEventBufferSize,omitempty" yaml:"defaultEventBufferSize,omitempty" validate:"omitempty,min=1" env:"DEFAULT_EVENT_BUFFER_SIZE"`

	// WorkerCount is the number of worker goroutines for async event processing.
	// These workers process events from asynchronous subscriptions. More workers
	// can increase throughput but also increase resource usage.
	// Must be at least 1. Used in single-engine mode.
	WorkerCount int `json:"workerCount,omitempty" yaml:"workerCount,omitempty" validate:"omitempty,min=1" env:"WORKER_COUNT"`

	// EventTTL is the time to live for events.
	// Events older than this value may be automatically removed from queues
	// or marked as expired. Used for event cleanup and storage management.
	EventTTL time.Duration `json:"eventTTL,omitempty" yaml:"eventTTL,omitempty" env:"EVENT_TTL" default:"3600s"`

	// RetentionDays is how many days to retain event history.
	// This affects event storage and cleanup policies. Longer retention
	// allows for event replay and debugging but requires more storage.
	// Must be at least 1. Used in single-engine mode.
	RetentionDays int `json:"retentionDays,omitempty" yaml:"retentionDays,omitempty" validate:"omitempty,min=1" env:"RETENTION_DAYS"`

	// ExternalBrokerURL is the connection URL for external message brokers.
	// Used when the engine is set to "redis", "kafka", or "kinesis". The format depends
	// on the specific broker type.
	// Examples:
	//   Redis: "redis://localhost:6379" or "redis://user:pass@host:port/db"
	//   Kafka: "kafka://localhost:9092" or "kafka://broker1:9092,broker2:9092"
	//   Kinesis: "https://kinesis.us-east-1.amazonaws.com"
	ExternalBrokerURL string `json:"externalBrokerURL,omitempty" yaml:"externalBrokerURL,omitempty" env:"EXTERNAL_BROKER_URL"`

	// ExternalBrokerUser is the username for external broker authentication.
	// Used when the external broker requires authentication.
	// Leave empty if the broker doesn't require authentication.
	ExternalBrokerUser string `json:"externalBrokerUser,omitempty" yaml:"externalBrokerUser,omitempty" env:"EXTERNAL_BROKER_USER"`

	// ExternalBrokerPassword is the password for external broker authentication.
	// Used when the external broker requires authentication.
	// Leave empty if the broker doesn't require authentication.
	// This should be kept secure and may be provided via environment variables.
	ExternalBrokerPassword string `json:"externalBrokerPassword,omitempty" yaml:"externalBrokerPassword,omitempty" env:"EXTERNAL_BROKER_PASSWORD"`

	// --- Multi-Engine Configuration (New) ---

	// Engines defines multiple event bus engines that can be used simultaneously.
	// When this field is populated, it takes precedence over the single-engine fields above.
	Engines []EngineConfig `json:"engines,omitempty" yaml:"engines,omitempty" validate:"dive"`

	// Routing defines how topics are routed to different engines.
	// Rules are evaluated in order, and the first matching rule is used.
	// If no routing rules are specified and multiple engines are configured,
	// all topics will be routed to the first engine.
	Routing []RoutingRule `json:"routing,omitempty" yaml:"routing,omitempty" validate:"dive"`
}

// IsMultiEngine returns true if this configuration uses multiple engines.
func (c *EventBusConfig) IsMultiEngine() bool {
	return len(c.Engines) > 0
}

// GetDefaultEngine returns the name of the default engine to use.
// For single-engine mode, returns "default".
// For multi-engine mode, returns the name of the first engine.
func (c *EventBusConfig) GetDefaultEngine() string {
	if c.IsMultiEngine() {
		if len(c.Engines) > 0 {
			return c.Engines[0].Name
		}
	}
	return "default"
}

// ValidateConfig performs additional validation on the configuration.
// This is called after basic struct tag validation.
func (c *EventBusConfig) ValidateConfig() error {
	if c.IsMultiEngine() {
		// Validate multi-engine configuration
		engineNames := make(map[string]bool)
		for _, engine := range c.Engines {
			if _, exists := engineNames[engine.Name]; exists {
				return fmt.Errorf("duplicate engine name: %s", engine.Name)
			}
			engineNames[engine.Name] = true
		}

		// Validate routing references existing engines
		for _, rule := range c.Routing {
			if _, exists := engineNames[rule.Engine]; !exists {
				return fmt.Errorf("routing rule references unknown engine: %s", rule.Engine)
			}
		}
	} else {
		// Validate single-engine configuration has required fields
		if c.Engine == "" {
			c.Engine = "memory" // Default value
		}
		if c.MaxEventQueueSize == 0 {
			c.MaxEventQueueSize = 1000 // Default value
		}
		if c.DefaultEventBufferSize == 0 {
			c.DefaultEventBufferSize = 10 // Default value
		}
		if c.WorkerCount == 0 {
			c.WorkerCount = 5 // Default value
		}
		if c.RetentionDays == 0 {
			c.RetentionDays = 7 // Default value
		}
		if c.EventTTL == 0 {
			c.EventTTL = time.Hour // Default value
		}
	}

	return nil
}
