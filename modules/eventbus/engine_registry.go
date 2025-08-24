package eventbus

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Static errors for engine registry
var (
	ErrUnknownEngineType    = errors.New("unknown engine type")
	ErrEngineNotFound       = errors.New("engine not found")
	ErrSubscriptionNotFound = errors.New("subscription not found in any engine")
)

// EngineFactory is a function that creates an EventBus implementation.
// It receives the engine configuration and returns a configured EventBus instance.
type EngineFactory func(config map[string]interface{}) (EventBus, error)

// engineRegistry manages the available engine types and their factories.
var engineRegistry = make(map[string]EngineFactory)

// RegisterEngine registers a new engine type with its factory function.
// This allows custom engines to be registered at runtime.
//
// Example:
//
//	eventbus.RegisterEngine("custom", func(config map[string]interface{}) (EventBus, error) {
//	    return NewCustomEngine(config), nil
//	})
func RegisterEngine(engineType string, factory EngineFactory) {
	engineRegistry[engineType] = factory
}

// GetRegisteredEngines returns a list of all registered engine types.
func GetRegisteredEngines() []string {
	engines := make([]string, 0, len(engineRegistry))
	for engineType := range engineRegistry {
		engines = append(engines, engineType)
	}
	return engines
}

// EngineRouter manages multiple event bus engines and routes events based on configuration.
type EngineRouter struct {
	engines       map[string]EventBus // Map of engine name to EventBus instance
	routing       []RoutingRule       // Routing rules in order of precedence
	defaultEngine string              // Default engine name for unmatched topics
}

// NewEngineRouter creates a new engine router with the given configuration.
func NewEngineRouter(config *EventBusConfig) (*EngineRouter, error) {
	router := &EngineRouter{
		engines:       make(map[string]EventBus),
		routing:       config.Routing,
		defaultEngine: config.GetDefaultEngine(),
	}

	if config.IsMultiEngine() {
		// Create engines from multi-engine configuration
		for _, engineConfig := range config.Engines {
			engine, err := createEngine(engineConfig.Type, engineConfig.Config)
			if err != nil {
				return nil, fmt.Errorf("failed to create engine %s (%s): %w",
					engineConfig.Name, engineConfig.Type, err)
			}
			router.engines[engineConfig.Name] = engine
		}
	} else {
		// Create single engine from legacy configuration
		engineConfig := map[string]interface{}{
			"maxEventQueueSize":      config.MaxEventQueueSize,
			"defaultEventBufferSize": config.DefaultEventBufferSize,
			"workerCount":            config.WorkerCount,
			"eventTTL":               config.EventTTL,
			"retentionDays":          config.RetentionDays,
			"externalBrokerURL":      config.ExternalBrokerURL,
			"externalBrokerUser":     config.ExternalBrokerUser,
			"externalBrokerPassword": config.ExternalBrokerPassword,
		}

		engine, err := createEngine(config.Engine, engineConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create engine %s: %w", config.Engine, err)
		}
		router.engines["default"] = engine
	}

	return router, nil
}

// createEngine creates an engine instance using the registered factory.
func createEngine(engineType string, config map[string]interface{}) (EventBus, error) {
	factory, exists := engineRegistry[engineType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEngineType, engineType)
	}

	return factory(config)
}

// SetModuleReference sets the module reference for all memory event buses
// This enables memory engines to emit events through the module
func (r *EngineRouter) SetModuleReference(module *EventBusModule) {
	for _, engine := range r.engines {
		if memoryEngine, ok := engine.(*MemoryEventBus); ok {
			memoryEngine.SetModule(module)
		}
	}
}

// Start starts all managed engines.
func (r *EngineRouter) Start(ctx context.Context) error {
	for name, engine := range r.engines {
		if err := engine.Start(ctx); err != nil {
			return fmt.Errorf("failed to start engine %s: %w", name, err)
		}
	}
	return nil
}

// Stop stops all managed engines.
func (r *EngineRouter) Stop(ctx context.Context) error {
	var lastError error
	for name, engine := range r.engines {
		if err := engine.Stop(ctx); err != nil {
			lastError = fmt.Errorf("failed to stop engine %s: %w", name, err)
		}
	}
	return lastError
}

// Publish publishes an event to the appropriate engine based on routing rules.
func (r *EngineRouter) Publish(ctx context.Context, event Event) error {
	engineName := r.getEngineForTopic(event.Topic)
	engine, exists := r.engines[engineName]
	if !exists {
		return fmt.Errorf("%w for topic %s: %s", ErrEngineNotFound, event.Topic, engineName)
	}

	if err := engine.Publish(ctx, event); err != nil {
		return fmt.Errorf("publishing to engine %s: %w", engineName, err)
	}
	return nil
}

// Subscribe subscribes to a topic using the appropriate engine.
// The subscription is created on the engine that handles the specified topic.
func (r *EngineRouter) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	engineName := r.getEngineForTopic(topic)
	engine, exists := r.engines[engineName]
	if !exists {
		return nil, fmt.Errorf("%w for topic %s: %s", ErrEngineNotFound, topic, engineName)
	}

	sub, err := engine.Subscribe(ctx, topic, handler)
	if err != nil {
		return nil, fmt.Errorf("subscribing to engine %s: %w", engineName, err)
	}
	return sub, nil
}

// SubscribeAsync subscribes to a topic asynchronously using the appropriate engine.
func (r *EngineRouter) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	engineName := r.getEngineForTopic(topic)
	engine, exists := r.engines[engineName]
	if !exists {
		return nil, fmt.Errorf("%w for topic %s: %s", ErrEngineNotFound, topic, engineName)
	}

	sub, err := engine.SubscribeAsync(ctx, topic, handler)
	if err != nil {
		return nil, fmt.Errorf("async subscribing to engine %s: %w", engineName, err)
	}
	return sub, nil
}

// Unsubscribe removes a subscription from its engine.
func (r *EngineRouter) Unsubscribe(ctx context.Context, subscription Subscription) error {
	// Try to unsubscribe from all engines - one of them should handle it
	for _, engine := range r.engines {
		err := engine.Unsubscribe(ctx, subscription)
		if err == nil {
			return nil
		}
		// Ignore errors for engines that don't have this subscription
	}
	return ErrSubscriptionNotFound
}

// Topics returns all active topics from all engines.
func (r *EngineRouter) Topics() []string {
	topicSet := make(map[string]bool)
	for _, engine := range r.engines {
		topics := engine.Topics()
		for _, topic := range topics {
			topicSet[topic] = true
		}
	}

	topics := make([]string, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}
	return topics
}

// SubscriberCount returns the total number of subscribers for a topic across all engines.
func (r *EngineRouter) SubscriberCount(topic string) int {
	total := 0
	for _, engine := range r.engines {
		total += engine.SubscriberCount(topic)
	}
	return total
}

// getEngineForTopic determines which engine should handle a given topic.
// It evaluates routing rules in order and returns the first match.
// If no rules match, it returns the default engine.
func (r *EngineRouter) getEngineForTopic(topic string) string {
	// Check routing rules in order
	for _, rule := range r.routing {
		for _, pattern := range rule.Topics {
			if r.topicMatches(topic, pattern) {
				return rule.Engine
			}
		}
	}

	// No routing rule matched, use default engine
	return r.defaultEngine
}

// topicMatches checks if a topic matches a pattern.
// Supports exact matches and wildcard patterns ending with '*'.
func (r *EngineRouter) topicMatches(topic, pattern string) bool {
	if topic == pattern {
		return true // Exact match
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(topic, prefix)
	}

	return false
}

// GetEngineNames returns the names of all configured engines.
func (r *EngineRouter) GetEngineNames() []string {
	names := make([]string, 0, len(r.engines))
	for name := range r.engines {
		names = append(names, name)
	}
	return names
}

// GetEngineForTopic returns the name of the engine that handles the specified topic.
// This is useful for debugging and monitoring.
func (r *EngineRouter) GetEngineForTopic(topic string) string {
	return r.getEngineForTopic(topic)
}

// init registers the built-in engine types.
func init() {
	// Register memory engine
	RegisterEngine("memory", func(config map[string]interface{}) (EventBus, error) {
		cfg := &EventBusConfig{
			MaxEventQueueSize:      1000,
			DefaultEventBufferSize: 10,
			WorkerCount:            5,
			RetentionDays:          7,
		}

		// Extract configuration values with defaults
		if val, ok := config["maxEventQueueSize"]; ok {
			if intVal, ok := val.(int); ok {
				cfg.MaxEventQueueSize = intVal
			}
		}
		if val, ok := config["defaultEventBufferSize"]; ok {
			if intVal, ok := val.(int); ok {
				cfg.DefaultEventBufferSize = intVal
			}
		}
		if val, ok := config["workerCount"]; ok {
			if intVal, ok := val.(int); ok {
				cfg.WorkerCount = intVal
			}
		}
		if val, ok := config["retentionDays"]; ok {
			if intVal, ok := val.(int); ok {
				cfg.RetentionDays = intVal
			}
		}

		return NewMemoryEventBus(cfg), nil
	})

	// Register Redis engine
	RegisterEngine("redis", NewRedisEventBus)

	// Register Kafka engine
	RegisterEngine("kafka", NewKafkaEventBus)

	// Register Kinesis engine
	RegisterEngine("kinesis", NewKinesisEventBus)

	// Register custom memory engine
	RegisterEngine("custom", NewCustomMemoryEventBus)
}
