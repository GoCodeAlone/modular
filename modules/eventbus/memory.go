package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryEventBus implements EventBus using in-memory channels
type MemoryEventBus struct {
	config         *EventBusConfig
	subscriptions  map[string]map[string]*memorySubscription
	topicMutex     sync.RWMutex
	workerPool     chan func()
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isStarted      bool
	eventHistory   map[string][]Event
	historyMutex   sync.RWMutex
	retentionTimer *time.Timer
}

// memorySubscription represents a subscription in the memory event bus
type memorySubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	eventCh   chan Event
	done      chan struct{}
	cancelled bool
	mutex     sync.RWMutex
}

// Topic returns the topic of the subscription
func (s *memorySubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *memorySubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *memorySubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *memorySubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	close(s.done)
	s.cancelled = true
	return nil
}

// NewMemoryEventBus creates a new in-memory event bus
func NewMemoryEventBus(config *EventBusConfig) *MemoryEventBus {
	return &MemoryEventBus{
		config:        config,
		subscriptions: make(map[string]map[string]*memorySubscription),
		eventHistory:  make(map[string][]Event),
	}
}

// Start initializes the event bus
func (m *MemoryEventBus) Start(ctx context.Context) error {
	if m.isStarted {
		return nil
	}

	m.ctx, m.cancel = context.WithCancel(ctx)

	// Initialize worker pool for async event handling
	m.workerPool = make(chan func(), m.config.WorkerCount)
	for i := 0; i < m.config.WorkerCount; i++ {
		m.wg.Add(1)
		go m.worker()
	}

	// Start retention timer to clean up old events
	m.startRetentionTimer()

	m.isStarted = true
	return nil
}

// Stop shuts down the event bus
func (m *MemoryEventBus) Stop(ctx context.Context) error {
	if !m.isStarted {
		return nil
	}

	// Cancel context to signal all workers to stop
	if m.cancel != nil {
		m.cancel()
	}

	// Stop retention timer
	if m.retentionTimer != nil {
		m.retentionTimer.Stop()
	}

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers exited gracefully
	case <-ctx.Done():
		return ErrEventBusShutdownTimedOut
	}

	m.isStarted = false
	return nil
}

// Publish sends an event to the specified topic
func (m *MemoryEventBus) Publish(ctx context.Context, event Event) error {
	if !m.isStarted {
		return ErrEventBusNotStarted
	}

	// Fill in event metadata
	event.CreatedAt = time.Now()
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	// Store in event history
	m.storeEventHistory(event)

	// Get subscribers for the topic
	m.topicMutex.RLock()
	subsMap, ok := m.subscriptions[event.Topic]

	// If no subscribers, just return
	if !ok || len(subsMap) == 0 {
		m.topicMutex.RUnlock()
		return nil
	}

	// Make a copy of the subscriptions to avoid holding the lock while publishing
	subs := make([]*memorySubscription, 0, len(subsMap))
	for _, sub := range subsMap {
		subs = append(subs, sub)
	}
	m.topicMutex.RUnlock()

	// Publish to all subscribers
	for _, sub := range subs {
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
			// Channel is full, log or handle as appropriate
		}
	}

	return nil
}

// Subscribe registers a handler for a topic
func (m *MemoryEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return m.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (m *MemoryEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return m.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (m *MemoryEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !m.isStarted {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Create a new subscription
	sub := &memorySubscription{
		id:        uuid.New().String(),
		topic:     topic,
		handler:   handler,
		isAsync:   isAsync,
		eventCh:   make(chan Event, m.config.DefaultEventBufferSize),
		done:      make(chan struct{}),
		cancelled: false,
	}

	// Add to subscriptions map
	m.topicMutex.Lock()
	if _, ok := m.subscriptions[topic]; !ok {
		m.subscriptions[topic] = make(map[string]*memorySubscription)
	}
	m.subscriptions[topic][sub.id] = sub
	m.topicMutex.Unlock()

	// Start event listener goroutine
	m.wg.Add(1)
	go m.handleEvents(sub)

	return sub, nil
}

// Unsubscribe removes a subscription
func (m *MemoryEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !m.isStarted {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*memorySubscription)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	// Cancel the subscription
	err := sub.Cancel()
	if err != nil {
		return err
	}

	// Remove from subscriptions map
	m.topicMutex.Lock()
	defer m.topicMutex.Unlock()

	if subs, ok := m.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(m.subscriptions, sub.topic)
		}
	}

	return nil
}

// Topics returns a list of all active topics
func (m *MemoryEventBus) Topics() []string {
	m.topicMutex.RLock()
	defer m.topicMutex.RUnlock()

	topics := make([]string, 0, len(m.subscriptions))
	for topic := range m.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (m *MemoryEventBus) SubscriberCount(topic string) int {
	m.topicMutex.RLock()
	defer m.topicMutex.RUnlock()

	if subs, ok := m.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// handleEvents processes events for a subscription
func (m *MemoryEventBus) handleEvents(sub *memorySubscription) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			// Event bus is shutting down
			return
		case <-sub.done:
			// Subscription was cancelled
			return
		case event := <-sub.eventCh:
			// Process the event
			if sub.isAsync {
				// For async subscriptions, queue the event handler in the worker pool
				m.queueEventHandler(sub, event)
			} else {
				// For sync subscriptions, process the event immediately
				now := time.Now()
				event.ProcessingStarted = &now

				// Process the event
				err := sub.handler(m.ctx, event)

				// Record completion
				completed := time.Now()
				event.ProcessingCompleted = &completed

				if err != nil {
					// Log error but continue processing
					slog.ErrorContext(m.ctx, "Event handler failed", "error", err, "topic", event.Topic)
				}
			}
		}
	}
}

// queueEventHandler adds an event handler to the worker pool
func (m *MemoryEventBus) queueEventHandler(sub *memorySubscription, event Event) {
	select {
	case m.workerPool <- func() {
		now := time.Now()
		event.ProcessingStarted = &now

		// Process the event
		err := sub.handler(m.ctx, event)

		// Record completion
		completed := time.Now()
		event.ProcessingCompleted = &completed

		if err != nil {
			// Log error but continue processing
			slog.ErrorContext(m.ctx, "Event handler failed", "error", err, "topic", event.Topic)
		}
	}:
		// Successfully queued
	default:
		// Worker pool is full, handle as appropriate
	}
}

// worker is a goroutine that processes events from the worker pool
func (m *MemoryEventBus) worker() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.workerPool:
			task()
		}
	}
}

// storeEventHistory adds an event to the history
func (m *MemoryEventBus) storeEventHistory(event Event) {
	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()

	if _, ok := m.eventHistory[event.Topic]; !ok {
		m.eventHistory[event.Topic] = make([]Event, 0)
	}

	// Add the event to history
	m.eventHistory[event.Topic] = append(m.eventHistory[event.Topic], event)
}

// startRetentionTimer starts a timer to clean up old events
func (m *MemoryEventBus) startRetentionTimer() {
	duration := 24 * time.Hour // Run cleanup once a day
	m.retentionTimer = time.AfterFunc(duration, func() {
		m.cleanupOldEvents()

		// Restart timer
		if m.isStarted {
			m.startRetentionTimer()
		}
	})
}

// cleanupOldEvents removes events older than retention period
func (m *MemoryEventBus) cleanupOldEvents() {
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)

	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()

	for topic, events := range m.eventHistory {
		filtered := make([]Event, 0, len(events))
		for _, event := range events {
			if event.CreatedAt.After(cutoff) {
				filtered = append(filtered, event)
			}
		}
		m.eventHistory[topic] = filtered
	}
}
