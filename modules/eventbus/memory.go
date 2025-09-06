package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GoCodeAlone/modular"
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
	module         *EventBusModule // Reference to emit events
	pubCounter     uint64          // for rotation fairness
	deliveredCount uint64          // stats
	droppedCount   uint64          // stats
}

// memorySubscription represents a subscription in the memory event bus
type memorySubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	eventCh   chan Event
	done      chan struct{}
	finished  chan struct{} // closed when handler goroutine exits
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

// isCancelled is a helper for internal fast path checks without exposing lock details
func (s *memorySubscription) isCancelled() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cancelled
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
		module:        nil, // Will be set when attached to a module
	}
}

// SetModule sets the parent module for event emission
func (m *MemoryEventBus) SetModule(module *EventBusModule) {
	m.module = module
}

// emitEvent emits an event through the module if available
func (m *MemoryEventBus) emitEvent(ctx context.Context, eventType, source string, data map[string]interface{}) {
	if m.module != nil {
		event := modular.NewCloudEvent(eventType, source, data, nil)
		go func() {
			if err := m.module.EmitEvent(ctx, event); err != nil {
				// Log but don't fail the operation
				slog.Debug("Failed to emit event", "type", eventType, "error", err)
			}
		}()
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
		return ErrEventBusShutdownTimeout
	}

	m.isStarted = false
	return nil
}

// matchesTopic checks if an event topic matches a subscription topic pattern
// Supports wildcard patterns like "user.*" matching "user.created", "user.updated", etc.
func matchesTopic(eventTopic, subscriptionTopic string) bool {
	// Exact match
	if eventTopic == subscriptionTopic {
		return true
	}

	// Wildcard match - check if subscription topic ends with *
	if len(subscriptionTopic) > 1 && subscriptionTopic[len(subscriptionTopic)-1] == '*' {
		prefix := subscriptionTopic[:len(subscriptionTopic)-1]
		return len(eventTopic) >= len(prefix) && eventTopic[:len(prefix)] == prefix
	}

	return false
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

	// Get all matching subscribers (exact match + wildcard matches)
	m.topicMutex.RLock()
	var allMatchingSubs []*memorySubscription

	// Check all subscription topics to find matches
	for subscriptionTopic, subsMap := range m.subscriptions {
		if matchesTopic(event.Topic, subscriptionTopic) {
			for _, sub := range subsMap {
				allMatchingSubs = append(allMatchingSubs, sub)
			}
		}
	}
	m.topicMutex.RUnlock()

	// If no matching subscribers, just return
	if len(allMatchingSubs) == 0 {
		return nil
	}

	// Optional rotation for fairness.
	// Rationale:
	//   * Deterministic order when rotation disabled (stable slice) improves testability and
	//     reasoning about delivery ordering.
	//   * When rotation enabled we perform a logical rotation using an incrementing counter
	//     rather than allocating + copying on every publish via append/slice tricks or
	//     performing a random shuffle. This yields O(n) copies only when the starting offset
	//     changes (and only for length > 1) with no RNG cost and avoids uint64->int casts
	//     that would require additional lint suppression.
	//   * Slice re-slicing with append could avoid an allocation in the start!=0 case, but the
	//     explicit copy keeps the code straightforward and side-effect free (no aliasing that
	//     could surprise future mutations) while cost is negligible relative to handler work.
	//   * We intentionally do not randomize: fairness over time is achieved by round‑robin
	//     style rotation (pubCounter % len) which ensures equal start positions statistically
	//     without introducing randomness into delivery order for reproducibility.
	// If performance profiling later shows this allocation hot, a specialized in-place rotate
	// could be introduced guarded by benchmarks.
	if m.config.RotateSubscriberOrder && len(allMatchingSubs) > 1 {
		pc := atomic.AddUint64(&m.pubCounter, 1) - 1
		ln := len(allMatchingSubs) // ln >= 2 here due to enclosing condition
		// Compute rotation starting offset. We keep start as uint64 and avoid any uint64->int cast
		// (gosec G115) by performing a manual copy instead of slicing with an int index.
		start64 := pc % uint64(ln)
		if start64 != 0 { // avoid allocation when rotation index is zero
			rotated := make([]*memorySubscription, 0, ln)
			// First copy from start64 to end
			for i := start64; i < uint64(ln); i++ {
				rotated = append(rotated, allMatchingSubs[i])
			}
			// Then copy from 0 to start64-1
			for i := uint64(0); i < start64; i++ {
				rotated = append(rotated, allMatchingSubs[i])
			}
			allMatchingSubs = rotated
		}
	}

	mode := m.config.DeliveryMode
	blockTimeout := m.config.PublishBlockTimeout

	for _, sub := range allMatchingSubs {
		sub.mutex.RLock()
		if sub.cancelled {
			sub.mutex.RUnlock()
			continue
		}
		sub.mutex.RUnlock()

		var sent bool
		switch mode {
		case "block":
			// block until space (respect context)
			select {
			case sub.eventCh <- event:
				sent = true
			case <-ctx.Done():
				// treat as drop due to cancellation
			}
		case "timeout":
			if blockTimeout <= 0 {
				// immediate attempt then drop
				select {
				case sub.eventCh <- event:
					sent = true
				default:
				}
			} else {
				deadline := time.NewTimer(blockTimeout)
				select {
				case sub.eventCh <- event:
					sent = true
				case <-deadline.C:
					// timeout drop
				case <-ctx.Done():
				}
				if !deadline.Stop() {
					<-deadline.C
				}
			}
		default: // "drop"
			select {
			case sub.eventCh <- event:
				sent = true
			default:
			}
		}
		// Only count drops at publish time; successful sends accounted when processed.
		if !sent {
			atomic.AddUint64(&m.droppedCount, 1)
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
		finished:  make(chan struct{}),
		cancelled: false,
	}

	// Add to subscriptions map
	m.topicMutex.Lock()
	isNewTopic := false
	if _, ok := m.subscriptions[topic]; !ok {
		m.subscriptions[topic] = make(map[string]*memorySubscription)
		isNewTopic = true
	}
	m.subscriptions[topic][sub.id] = sub
	m.topicMutex.Unlock()

	// Emit topic created event if this is a new topic
	if isNewTopic {
		m.emitEvent(ctx, EventTypeTopicCreated, "memory-eventbus", map[string]interface{}{
			"topic": topic,
		})
	}

	// Start event listener goroutine and wait for it to be ready
	started := make(chan struct{})
	m.wg.Add(1)
	go func() {
		close(started) // Signal that the goroutine has started
		m.handleEvents(sub)
	}()

	// Wait for the goroutine to be ready before returning
	<-started

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

	// Cancel the subscription (sets cancelled flag and closes done channel)
	if err := sub.Cancel(); err != nil {
		return err
	}

	// Remove from subscriptions map
	m.topicMutex.Lock()
	topicDeleted := false
	if subs, ok := m.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(m.subscriptions, sub.topic)
			topicDeleted = true
		}
	}
	m.topicMutex.Unlock()

	// Wait (briefly) for handler goroutine to terminate to avoid post-unsubscribe deliveries
	select {
	case <-sub.finished:
	case <-time.After(100 * time.Millisecond):
	}

	if topicDeleted {
		m.emitEvent(ctx, EventTypeTopicDeleted, "memory-eventbus", map[string]interface{}{
			"topic": sub.topic,
		})
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
	defer close(sub.finished)

	for {
		// Fast path: if subscription cancelled, exit before selecting (avoids processing backlog after unsubscribe)
		if sub.isCancelled() {
			return
		}
		select {
		case <-m.ctx.Done():
			return
		case <-sub.done:
			return
		case event := <-sub.eventCh:
			// Re-check cancellation after dequeue to avoid processing additional events post-unsubscribe.
			if sub.isCancelled() {
				return
			}
			if sub.isAsync {
				m.queueEventHandler(sub, event)
				continue
			}
			now := time.Now()
			event.ProcessingStarted = &now
			m.emitEvent(m.ctx, EventTypeMessageReceived, "memory-eventbus", map[string]interface{}{
				"topic":           event.Topic,
				"subscription_id": sub.id,
			})
			err := sub.handler(m.ctx, event)
			completed := time.Now()
			event.ProcessingCompleted = &completed
			if err != nil {
				m.emitEvent(m.ctx, EventTypeMessageFailed, "memory-eventbus", map[string]interface{}{
					"topic":           event.Topic,
					"subscription_id": sub.id,
					"error":           err.Error(),
				})
				slog.Error("Event handler failed", "error", err, "topic", event.Topic)
			}
			atomic.AddUint64(&m.deliveredCount, 1)
		}
	}
}

// queueEventHandler adds an event handler to the worker pool
func (m *MemoryEventBus) queueEventHandler(sub *memorySubscription, event Event) {
	select {
	case m.workerPool <- func() {
		now := time.Now()
		event.ProcessingStarted = &now

		// Emit message received event
		m.emitEvent(m.ctx, EventTypeMessageReceived, "memory-eventbus", map[string]interface{}{
			"topic":           event.Topic,
			"subscription_id": sub.id,
		})

		// Process the event
		err := sub.handler(m.ctx, event)

		// Record completion
		completed := time.Now()
		event.ProcessingCompleted = &completed

		if err != nil {
			// Emit message failed event for handler errors
			m.emitEvent(m.ctx, EventTypeMessageFailed, "memory-eventbus", map[string]interface{}{
				"topic":           event.Topic,
				"subscription_id": sub.id,
				"error":           err.Error(),
			})
			// Log error but continue processing
			slog.Error("Event handler failed", "error", err, "topic", event.Topic)
		}
		// Count as delivered after processing (success or failure)
		atomic.AddUint64(&m.deliveredCount, 1)
	}:
		// Successfully queued; delivered count increment deferred until post-processing
	default:
		// Worker pool is full, drop async processing (count as dropped)
		atomic.AddUint64(&m.droppedCount, 1)
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

// Stats returns basic delivery stats for monitoring/testing.
func (m *MemoryEventBus) Stats() (delivered uint64, dropped uint64) {
	return atomic.LoadUint64(&m.deliveredCount), atomic.LoadUint64(&m.droppedCount)
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
