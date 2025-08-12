package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CustomMemoryEventBus is an example custom implementation of the EventBus interface.
// This demonstrates how to create and register custom engines. Unlike the standard
// memory engine, this one includes additional features like event metrics collection,
// custom event filtering, and enhanced subscription management.
type CustomMemoryEventBus struct {
	config         *CustomMemoryConfig
	subscriptions  map[string]map[string]*customMemorySubscription
	topicMutex     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	isStarted      bool
	eventMetrics   *EventMetrics
	eventFilters   []EventFilter
}

// CustomMemoryConfig holds configuration for the custom memory engine
type CustomMemoryConfig struct {
	MaxEventQueueSize      int                    `json:"maxEventQueueSize"`
	DefaultEventBufferSize int                    `json:"defaultEventBufferSize"`
	EnableMetrics          bool                   `json:"enableMetrics"`
	MetricsInterval        time.Duration          `json:"metricsInterval"`
	EventFilters           []map[string]interface{} `json:"eventFilters"`
}

// EventMetrics holds metrics about event processing
type EventMetrics struct {
	TotalEvents          int64         `json:"totalEvents"`
	EventsPerTopic       map[string]int64 `json:"eventsPerTopic"`
	AverageProcessingTime time.Duration `json:"averageProcessingTime"`
	LastResetTime        time.Time     `json:"lastResetTime"`
	mutex                sync.RWMutex
}

// EventFilter defines a filter that can be applied to events
type EventFilter interface {
	ShouldProcess(event Event) bool
	Name() string
}

// TopicPrefixFilter filters events based on topic prefix
type TopicPrefixFilter struct {
	AllowedPrefixes []string
	name            string
}

func (f *TopicPrefixFilter) ShouldProcess(event Event) bool {
	if len(f.AllowedPrefixes) == 0 {
		return true // No filtering if no prefixes specified
	}

	for _, prefix := range f.AllowedPrefixes {
		if len(event.Topic) >= len(prefix) && event.Topic[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (f *TopicPrefixFilter) Name() string {
	return f.name
}

// customMemorySubscription represents a subscription in the custom memory event bus
type customMemorySubscription struct {
	id               string
	topic            string
	handler          EventHandler
	isAsync          bool
	eventCh          chan Event
	done             chan struct{}
	cancelled        bool
	mutex            sync.RWMutex
	subscriptionTime time.Time
	processedEvents  int64
}

// Topic returns the topic of the subscription
func (s *customMemorySubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *customMemorySubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *customMemorySubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *customMemorySubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	close(s.done)
	s.cancelled = true
	return nil
}

// ProcessedEvents returns the number of events processed by this subscription
func (s *customMemorySubscription) ProcessedEvents() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.processedEvents
}

// NewCustomMemoryEventBus creates a new custom memory-based event bus
func NewCustomMemoryEventBus(config map[string]interface{}) (EventBus, error) {
	customConfig := &CustomMemoryConfig{
		MaxEventQueueSize:      1000,
		DefaultEventBufferSize: 10,
		EnableMetrics:          true,
		MetricsInterval:        30 * time.Second,
		EventFilters:           make([]map[string]interface{}, 0),
	}

	// Parse configuration
	if val, ok := config["maxEventQueueSize"]; ok {
		if intVal, ok := val.(int); ok {
			customConfig.MaxEventQueueSize = intVal
		}
	}
	if val, ok := config["defaultEventBufferSize"]; ok {
		if intVal, ok := val.(int); ok {
			customConfig.DefaultEventBufferSize = intVal
		}
	}
	if val, ok := config["enableMetrics"]; ok {
		if boolVal, ok := val.(bool); ok {
			customConfig.EnableMetrics = boolVal
		}
	}
	if val, ok := config["metricsInterval"]; ok {
		if strVal, ok := val.(string); ok {
			if duration, err := time.ParseDuration(strVal); err == nil {
				customConfig.MetricsInterval = duration
			}
		}
	}

	eventMetrics := &EventMetrics{
		EventsPerTopic: make(map[string]int64),
		LastResetTime:  time.Now(),
	}

	bus := &CustomMemoryEventBus{
		config:        customConfig,
		subscriptions: make(map[string]map[string]*customMemorySubscription),
		eventMetrics:  eventMetrics,
		eventFilters:  make([]EventFilter, 0),
	}

	// Initialize event filters based on configuration
	for _, filterConfig := range customConfig.EventFilters {
		if filterType, ok := filterConfig["type"].(string); ok && filterType == "topicPrefix" {
			if prefixes, ok := filterConfig["prefixes"].([]interface{}); ok {
				allowedPrefixes := make([]string, len(prefixes))
				for i, prefix := range prefixes {
					allowedPrefixes[i] = prefix.(string)
				}
				filter := &TopicPrefixFilter{
					AllowedPrefixes: allowedPrefixes,
					name:           "topicPrefix",
				}
				bus.eventFilters = append(bus.eventFilters, filter)
			}
		}
	}

	return bus, nil
}

// Start initializes the custom memory event bus
func (c *CustomMemoryEventBus) Start(ctx context.Context) error {
	if c.isStarted {
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Start metrics collection if enabled
	if c.config.EnableMetrics {
		go c.metricsCollector()
	}

	c.isStarted = true
	slog.Info("Custom memory event bus started with enhanced features",
		"metricsEnabled", c.config.EnableMetrics,
		"filterCount", len(c.eventFilters))
	return nil
}

// Stop shuts down the custom memory event bus
func (c *CustomMemoryEventBus) Stop(ctx context.Context) error {
	if !c.isStarted {
		return nil
	}

	// Cancel context to signal all workers to stop
	if c.cancel != nil {
		c.cancel()
	}

	// Cancel all subscriptions
	c.topicMutex.Lock()
	for _, subs := range c.subscriptions {
		for _, sub := range subs {
			sub.Cancel()
		}
	}
	c.topicMutex.Unlock()

	c.isStarted = false
	slog.Info("Custom memory event bus stopped",
		"totalEvents", c.eventMetrics.TotalEvents,
		"topics", len(c.eventMetrics.EventsPerTopic))
	return nil
}

// Publish sends an event to the specified topic with custom filtering and metrics
func (c *CustomMemoryEventBus) Publish(ctx context.Context, event Event) error {
	if !c.isStarted {
		return ErrEventBusNotStarted
	}

	// Apply event filters
	for _, filter := range c.eventFilters {
		if !filter.ShouldProcess(event) {
			slog.Debug("Event filtered out", "topic", event.Topic, "filter", filter.Name())
			return nil // Event filtered out
		}
	}

	// Fill in event metadata
	event.CreatedAt = time.Now()
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}
	event.Metadata["engine"] = "custom-memory"

	// Update metrics
	if c.config.EnableMetrics {
		c.eventMetrics.mutex.Lock()
		c.eventMetrics.TotalEvents++
		c.eventMetrics.EventsPerTopic[event.Topic]++
		c.eventMetrics.mutex.Unlock()
	}

	// Get all matching subscribers
	c.topicMutex.RLock()
	var allMatchingSubs []*customMemorySubscription

	for subscriptionTopic, subsMap := range c.subscriptions {
		if c.matchesTopic(event.Topic, subscriptionTopic) {
			for _, sub := range subsMap {
				allMatchingSubs = append(allMatchingSubs, sub)
			}
		}
	}
	c.topicMutex.RUnlock()

	// Publish to all matching subscribers
	for _, sub := range allMatchingSubs {
		sub.mutex.RLock()
		if sub.cancelled {
			sub.mutex.RUnlock()
			continue
		}
		sub.mutex.RUnlock()

		select {
		case sub.eventCh <- event:
			// Event sent to subscriber
		default:
			// Channel is full, log warning
			slog.Warn("Subscription channel full, dropping event",
				"topic", event.Topic, "subscriptionID", sub.id)
		}
	}

	return nil
}

// Subscribe registers a handler for a topic
func (c *CustomMemoryEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return c.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (c *CustomMemoryEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return c.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (c *CustomMemoryEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !c.isStarted {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Create a new subscription with enhanced features
	sub := &customMemorySubscription{
		id:               uuid.New().String(),
		topic:            topic,
		handler:          handler,
		isAsync:          isAsync,
		eventCh:          make(chan Event, c.config.DefaultEventBufferSize),
		done:             make(chan struct{}),
		cancelled:        false,
		subscriptionTime: time.Now(),
		processedEvents:  0,
	}

	// Add to subscriptions map
	c.topicMutex.Lock()
	if _, ok := c.subscriptions[topic]; !ok {
		c.subscriptions[topic] = make(map[string]*customMemorySubscription)
	}
	c.subscriptions[topic][sub.id] = sub
	c.topicMutex.Unlock()

	// Start event handler goroutine
	go c.handleEvents(sub)

	slog.Debug("Created custom subscription", "topic", topic, "id", sub.id, "async", isAsync)
	return sub, nil
}

// Unsubscribe removes a subscription
func (c *CustomMemoryEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !c.isStarted {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*customMemorySubscription)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	// Log subscription statistics
	slog.Debug("Unsubscribing custom subscription",
		"topic", sub.topic,
		"id", sub.id,
		"processedEvents", sub.ProcessedEvents(),
		"duration", time.Since(sub.subscriptionTime))

	// Cancel the subscription
	err := sub.Cancel()
	if err != nil {
		return err
	}

	// Remove from subscriptions map
	c.topicMutex.Lock()
	defer c.topicMutex.Unlock()

	if subs, ok := c.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(c.subscriptions, sub.topic)
		}
	}

	return nil
}

// Topics returns a list of all active topics
func (c *CustomMemoryEventBus) Topics() []string {
	c.topicMutex.RLock()
	defer c.topicMutex.RUnlock()

	topics := make([]string, 0, len(c.subscriptions))
	for topic := range c.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (c *CustomMemoryEventBus) SubscriberCount(topic string) int {
	c.topicMutex.RLock()
	defer c.topicMutex.RUnlock()

	if subs, ok := c.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// matchesTopic checks if an event topic matches a subscription topic pattern
func (c *CustomMemoryEventBus) matchesTopic(eventTopic, subscriptionTopic string) bool {
	// Exact match
	if eventTopic == subscriptionTopic {
		return true
	}

	// Wildcard match
	if len(subscriptionTopic) > 1 && subscriptionTopic[len(subscriptionTopic)-1] == '*' {
		prefix := subscriptionTopic[:len(subscriptionTopic)-1]
		return len(eventTopic) >= len(prefix) && eventTopic[:len(prefix)] == prefix
	}

	return false
}

// handleEvents processes events for a custom subscription
func (c *CustomMemoryEventBus) handleEvents(sub *customMemorySubscription) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-sub.done:
			return
		case event := <-sub.eventCh:
			startTime := time.Now()
			event.ProcessingStarted = &startTime

			// Process the event
			err := sub.handler(c.ctx, event)

			// Record completion and metrics
			completedTime := time.Now()
			event.ProcessingCompleted = &completedTime
			processingDuration := completedTime.Sub(startTime)

			// Update subscription metrics
			sub.mutex.Lock()
			sub.processedEvents++
			sub.mutex.Unlock()

			// Update global metrics
			if c.config.EnableMetrics {
				c.eventMetrics.mutex.Lock()
				// Simple moving average for processing time
				c.eventMetrics.AverageProcessingTime = 
					(c.eventMetrics.AverageProcessingTime + processingDuration) / 2
				c.eventMetrics.mutex.Unlock()
			}

			if err != nil {
				slog.Error("Custom memory event handler failed",
					"error", err,
					"topic", event.Topic,
					"subscriptionID", sub.id,
					"processingDuration", processingDuration)
			}
		}
	}
}

// metricsCollector periodically logs metrics
func (c *CustomMemoryEventBus) metricsCollector() {
	ticker := time.NewTicker(c.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.logMetrics()
		}
	}
}

// logMetrics logs current event bus metrics
func (c *CustomMemoryEventBus) logMetrics() {
	c.eventMetrics.mutex.RLock()
	totalEvents := c.eventMetrics.TotalEvents
	eventsPerTopic := make(map[string]int64)
	for k, v := range c.eventMetrics.EventsPerTopic {
		eventsPerTopic[k] = v
	}
	avgProcessingTime := c.eventMetrics.AverageProcessingTime
	c.eventMetrics.mutex.RUnlock()

	c.topicMutex.RLock()
	activeTopics := len(c.subscriptions)
	totalSubscriptions := 0
	for _, subs := range c.subscriptions {
		totalSubscriptions += len(subs)
	}
	c.topicMutex.RUnlock()

	slog.Info("Custom memory event bus metrics",
		"totalEvents", totalEvents,
		"activeTopics", activeTopics,
		"totalSubscriptions", totalSubscriptions,
		"avgProcessingTime", avgProcessingTime,
		"eventsPerTopic", eventsPerTopic)
}

// GetMetrics returns current event metrics (additional method not in EventBus interface)
func (c *CustomMemoryEventBus) GetMetrics() *EventMetrics {
	c.eventMetrics.mutex.RLock()
	defer c.eventMetrics.mutex.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := &EventMetrics{
		TotalEvents:           c.eventMetrics.TotalEvents,
		EventsPerTopic:        make(map[string]int64),
		AverageProcessingTime: c.eventMetrics.AverageProcessingTime,
		LastResetTime:         c.eventMetrics.LastResetTime,
	}
	
	for k, v := range c.eventMetrics.EventsPerTopic {
		metrics.EventsPerTopic[k] = v
	}
	
	return metrics
}