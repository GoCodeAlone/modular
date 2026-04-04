package eventbus

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ErrDurableQueueClosed is returned by Push when the subscription has been cancelled.
var ErrDurableQueueClosed = errors.New("durable queue closed")

// durableQueue is a goroutine-safe, bounded FIFO queue backed by a linked list.
//
// Push adds items and blocks when maxDepth > 0 and the queue is at capacity
// (backpressure), instead of dropping items. TryPop removes items without
// blocking; it is paired with a Notify channel that signals when new items are
// available, allowing callers to use a select rather than busy-waiting.
//
// The zero maxDepth means unlimited capacity (use with caution: unbounded growth).
type durableQueue struct {
	mu       sync.Mutex
	items    *list.List
	maxDepth int           // 0 = unlimited
	notEmpty chan struct{} // buffered(1); signaled on Push so consumer can wake
	notFull  chan struct{} // buffered(1); signaled on TryPop from a full queue
}

func newDurableQueue(maxDepth int) *durableQueue {
	return &durableQueue{
		items:    list.New(),
		maxDepth: maxDepth,
		notEmpty: make(chan struct{}, 1),
		notFull:  make(chan struct{}, 1),
	}
}

// Push enqueues event. If maxDepth > 0 and the queue is already at capacity,
// Push blocks until a slot is freed, ctx is cancelled, or done is closed.
// Returns ctx.Err() on cancellation or ErrDurableQueueClosed when done is closed.
func (q *durableQueue) Push(ctx context.Context, done <-chan struct{}, event Event) error {
	for {
		q.mu.Lock()
		if q.maxDepth <= 0 || q.items.Len() < q.maxDepth {
			q.items.PushBack(event)
			// Non-blocking signal: wake at most one waiting TryPop caller.
			select {
			case q.notEmpty <- struct{}{}:
			default:
			}
			q.mu.Unlock()
			return nil
		}
		q.mu.Unlock()

		// Queue is at capacity; apply backpressure.
		select {
		case <-ctx.Done():
			return fmt.Errorf("publish cancelled while waiting for queue space: %w", ctx.Err())
		case <-done:
			return ErrDurableQueueClosed
		case <-q.notFull:
			// A slot was freed; retry.
		}
	}
}

// TryPop removes and returns the front item without blocking.
// Returns (event, true) on success or (zero, false) when the queue is empty.
func (q *durableQueue) TryPop() (Event, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	front := q.items.Front()
	if front == nil {
		return Event{}, false
	}

	// Only wake a blocked Push when we pop from a full queue to avoid spurious wakeups.
	wasAtCapacity := q.maxDepth > 0 && q.items.Len() >= q.maxDepth
	event := q.items.Remove(front).(Event) //nolint:forcetypeassert // durableQueue only stores Event values; type assertion is guaranteed safe
	if wasAtCapacity {
		select {
		case q.notFull <- struct{}{}:
		default:
		}
	}
	return event, true
}

// Notify returns the channel that is signaled whenever an item is pushed.
// Callers should loop back to TryPop after receiving from this channel since
// the notification is a hint, not a 1:1 guarantee per item.
func (q *durableQueue) Notify() <-chan struct{} {
	return q.notEmpty
}

// Len returns the current number of queued items.
func (q *durableQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

// durableSub is a subscription backed by a durableQueue.
type durableSub struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	queue     *durableQueue
	done      chan struct{} // closed by Cancel
	finished  chan struct{} // closed when handleEvents exits
	cancelled bool
	mutex     sync.RWMutex
}

func (s *durableSub) Topic() string { return s.topic }
func (s *durableSub) ID() string    { return s.id }
func (s *durableSub) IsAsync() bool { return s.isAsync }

func (s *durableSub) isCancelled() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.cancelled
}

// Cancel closes the subscription.
func (s *durableSub) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancelled {
		return nil
	}
	close(s.done)
	s.cancelled = true
	return nil
}

// DurableMemoryEventBus is an in-process event bus that guarantees delivery by
// applying backpressure to publishers instead of dropping events.
//
// Every subscriber gets a dedicated FIFO queue backed by a linked list. When the
// queue reaches MaxDurableQueueDepth, the publishing goroutine blocks until the
// subscriber consumes an event. This bounds memory usage while ensuring zero event
// loss under normal operation.
//
// Comparison with MemoryEventBus:
//   - Guarantee:    zero event loss       vs  possible silent drops
//   - Overflow:     publisher blocks      vs  events dropped (or dropped after timeout)
//   - Memory:       bounded by MaxDurableQueueDepth × subscribers
//   - Async mode:   handlers run inline in the dispatch loop; "async" flag is stored
//     for API symmetry but all handlers execute sequentially per subscription
//
// For cross-process or crash-durable persistence, use an external engine (Redis, Kafka).
// Configure with engine type "durable-memory":
//
//	eventbus:
//	  engine: durable-memory
//	  maxDurableQueueDepth: 500   # per-subscriber; 0 = MaxEventQueueSize
type DurableMemoryEventBus struct {
	config         *EventBusConfig
	subscriptions  map[string]map[string]*durableSub
	topicMutex     sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isStarted      atomic.Bool
	module         *EventBusModule
	deliveredCount uint64
}

// NewDurableMemoryEventBus is the engine factory for "durable-memory".
func NewDurableMemoryEventBus(config map[string]interface{}) (EventBus, error) {
	cfg := &EventBusConfig{
		MaxEventQueueSize:    1000,
		RetentionDays:        7,
		MaxDurableQueueDepth: 0, // default: use MaxEventQueueSize
	}

	if val, ok := config["maxEventQueueSize"]; ok {
		if intVal, ok := val.(int); ok {
			cfg.MaxEventQueueSize = intVal
		}
	}
	if val, ok := config["maxDurableQueueDepth"]; ok {
		if intVal, ok := val.(int); ok {
			cfg.MaxDurableQueueDepth = intVal
		}
	}
	if val, ok := config["retentionDays"]; ok {
		if intVal, ok := val.(int); ok {
			cfg.RetentionDays = intVal
		}
	}

	return &DurableMemoryEventBus{
		config:        cfg,
		subscriptions: make(map[string]map[string]*durableSub),
	}, nil
}

// SetModule wires the parent EventBusModule so the engine can emit internal events.
func (d *DurableMemoryEventBus) SetModule(module *EventBusModule) {
	d.module = module
}

// Start initialises the durable memory event bus.
func (d *DurableMemoryEventBus) Start(ctx context.Context) error {
	if d.isStarted.Load() {
		return nil
	}
	d.ctx, d.cancel = context.WithCancel(ctx) //nolint:gosec // G118: cancel is stored in d.cancel and called in Stop()
	d.isStarted.Store(true)
	return nil
}

// Stop shuts down the durable memory event bus.
// All active subscriber goroutines are signalled to stop and the method waits
// until they exit (or ctx expires).
func (d *DurableMemoryEventBus) Stop(ctx context.Context) error {
	if !d.isStarted.Load() {
		return nil
	}

	if d.cancel != nil {
		d.cancel()
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in durable memory eventbus shutdown waiter", "error", r)
			}
		}()
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ErrEventBusShutdownTimeout
	}

	d.isStarted.Store(false)
	return nil
}

// queueDepth returns the per-subscriber queue capacity.
// MaxDurableQueueDepth takes precedence; falls back to MaxEventQueueSize.
func (d *DurableMemoryEventBus) queueDepth() int {
	if d.config.MaxDurableQueueDepth > 0 {
		return d.config.MaxDurableQueueDepth
	}
	return d.config.MaxEventQueueSize
}

// Publish sends event to all matching subscribers.
// If any subscriber's queue is full, Publish blocks that subscriber's delivery
// until space is available (backpressure), then continues to the next subscriber.
// Returns ctx.Err() if the context is cancelled before all subscribers are reached.
func (d *DurableMemoryEventBus) Publish(ctx context.Context, event Event) error {
	if !d.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	if event.Time().IsZero() {
		event.SetTime(time.Now())
	}

	d.topicMutex.RLock()
	var subs []*durableSub
	for subTopic, subsMap := range d.subscriptions {
		if matchesTopic(event.Type(), subTopic) {
			for _, s := range subsMap {
				subs = append(subs, s)
			}
		}
	}
	d.topicMutex.RUnlock()

	for _, sub := range subs {
		if sub.isCancelled() {
			continue
		}
		if err := sub.queue.Push(ctx, sub.done, event); err != nil {
			if errors.Is(err, ErrDurableQueueClosed) {
				// Subscription was cancelled while we were pushing; skip.
				continue
			}
			// Context cancelled — propagate to caller.
			return err
		}
	}
	return nil
}

// Subscribe registers a synchronous handler for topic.
func (d *DurableMemoryEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return d.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for topic.
// In the durable engine, async and sync subscriptions share the same delivery path
// (inline in the per-subscription dispatch goroutine) to preserve the zero-loss
// guarantee without introducing a shared worker pool that could become a bottleneck.
func (d *DurableMemoryEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return d.subscribe(ctx, topic, handler, true)
}

func (d *DurableMemoryEventBus) subscribe(_ context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !d.isStarted.Load() {
		return nil, ErrEventBusNotStarted
	}
	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	sub := &durableSub{
		id:       uuid.New().String(),
		topic:    topic,
		handler:  handler,
		isAsync:  isAsync,
		queue:    newDurableQueue(d.queueDepth()),
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}

	d.topicMutex.Lock()
	if _, ok := d.subscriptions[topic]; !ok {
		d.subscriptions[topic] = make(map[string]*durableSub)
	}
	d.subscriptions[topic][sub.id] = sub
	d.topicMutex.Unlock()

	started := make(chan struct{})
	d.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("handleEvents panic recovered", "error", r, "topic", sub.topic)
			}
		}()
		close(started)
		d.handleEvents(sub)
	}()
	<-started // wait until goroutine is scheduled before returning

	return sub, nil
}

// Unsubscribe cancels a subscription and removes it from the bus.
func (d *DurableMemoryEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !d.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*durableSub)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	if err := sub.Cancel(); err != nil {
		return err
	}

	d.topicMutex.Lock()
	if subs, ok := d.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(d.subscriptions, sub.topic)
		}
	}
	d.topicMutex.Unlock()

	// Brief wait for the handler goroutine to exit before returning.
	t := time.NewTimer(100 * time.Millisecond)
	defer t.Stop()
	select {
	case <-sub.finished:
	case <-t.C:
	}

	return nil
}

// Topics returns all topic names that currently have at least one subscriber.
func (d *DurableMemoryEventBus) Topics() []string {
	d.topicMutex.RLock()
	defer d.topicMutex.RUnlock()

	topics := make([]string, 0, len(d.subscriptions))
	for t := range d.subscriptions {
		topics = append(topics, t)
	}
	return topics
}

// SubscriberCount returns the number of active subscribers for topic.
func (d *DurableMemoryEventBus) SubscriberCount(topic string) int {
	d.topicMutex.RLock()
	defer d.topicMutex.RUnlock()
	return len(d.subscriptions[topic])
}

// Stats returns the total number of events delivered by this engine.
func (d *DurableMemoryEventBus) Stats() (delivered uint64) {
	return atomic.LoadUint64(&d.deliveredCount)
}

// handleEvents is the per-subscription event dispatch loop.
// It drains the subscription's durableQueue and invokes the handler for each event.
// The loop exits when the bus context is cancelled or the subscription is cancelled.
func (d *DurableMemoryEventBus) handleEvents(sub *durableSub) {
	defer d.wg.Done()
	defer close(sub.finished)

	for {
		if sub.isCancelled() {
			return
		}

		// Fast path: drain any available events without blocking.
		if event, ok := sub.queue.TryPop(); ok {
			if sub.isCancelled() {
				return
			}
			err := sub.handler(d.ctx, event)
			if err != nil {
				slog.Error("Durable event handler failed",
					"error", err,
					"topic", event.Type(),
					"subscription_id", sub.id)
			}
			atomic.AddUint64(&d.deliveredCount, 1)
			continue
		}

		// Queue is empty; wait for the next push notification or shutdown.
		select {
		case <-d.ctx.Done():
			return
		case <-sub.done:
			return
		case <-sub.queue.Notify():
			// An item was pushed; loop and try TryPop again.
		}
	}
}
