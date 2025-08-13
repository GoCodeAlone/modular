package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// RedisEventBus implements EventBus using Redis pub/sub
type RedisEventBus struct {
	config        *RedisConfig
	client        *redis.Client
	subscriptions map[string]map[string]*redisSubscription
	topicMutex    sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	isStarted     bool
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	URL      string `json:"url"`
	DB       int    `json:"db"`
	Username string `json:"username"`
	Password string `json:"password"`
	PoolSize int    `json:"poolSize"`
}

// redisSubscription represents a subscription in the Redis event bus
type redisSubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	pubsub    *redis.PubSub
	done      chan struct{}
	cancelled bool
	mutex     sync.RWMutex
	bus       *RedisEventBus
}

// Topic returns the topic of the subscription
func (s *redisSubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *redisSubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *redisSubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *redisSubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	s.cancelled = true
	if s.pubsub != nil {
		s.pubsub.Close()
	}
	close(s.done)
	return nil
}

// NewRedisEventBus creates a new Redis-based event bus
func NewRedisEventBus(config map[string]interface{}) (EventBus, error) {
	redisConfig := &RedisConfig{
		URL:      "redis://localhost:6379",
		DB:       0,
		PoolSize: 10,
	}

	// Parse configuration
	if url, ok := config["url"].(string); ok {
		redisConfig.URL = url
	}
	if db, ok := config["db"].(int); ok {
		redisConfig.DB = db
	}
	if username, ok := config["username"].(string); ok {
		redisConfig.Username = username
	}
	if password, ok := config["password"].(string); ok {
		redisConfig.Password = password
	}
	if poolSize, ok := config["poolSize"].(int); ok {
		redisConfig.PoolSize = poolSize
	}

	// Parse Redis connection URL
	opts, err := redis.ParseURL(redisConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	// Override with explicit config
	opts.DB = redisConfig.DB
	opts.PoolSize = redisConfig.PoolSize
	if redisConfig.Username != "" {
		opts.Username = redisConfig.Username
	}
	if redisConfig.Password != "" {
		opts.Password = redisConfig.Password
	}

	client := redis.NewClient(opts)

	return &RedisEventBus{
		config:        redisConfig,
		client:        client,
		subscriptions: make(map[string]map[string]*redisSubscription),
	}, nil
}

// Start initializes the Redis event bus
func (r *RedisEventBus) Start(ctx context.Context) error {
	if r.isStarted {
		return nil
	}

	// Test connection
	_, err := r.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.isStarted = true
	return nil
}

// Stop shuts down the Redis event bus
func (r *RedisEventBus) Stop(ctx context.Context) error {
	if !r.isStarted {
		return nil
	}

	// Cancel context to signal all workers to stop
	if r.cancel != nil {
		r.cancel()
	}

	// Cancel all subscriptions
	r.topicMutex.Lock()
	for _, subs := range r.subscriptions {
		for _, sub := range subs {
			_ = sub.Cancel() // Ignore error during shutdown
		}
	}
	r.subscriptions = make(map[string]map[string]*redisSubscription)
	r.topicMutex.Unlock()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers exited gracefully
	case <-ctx.Done():
		return ErrEventBusShutdownTimeout
	}

	// Close Redis client
	if err := r.client.Close(); err != nil {
		return fmt.Errorf("error closing Redis client: %w", err)
	}

	r.isStarted = false
	return nil
}

// Publish sends an event to the specified topic using Redis pub/sub
func (r *RedisEventBus) Publish(ctx context.Context, event Event) error {
	if !r.isStarted {
		return ErrEventBusNotStarted
	}

	// Fill in event metadata
	event.CreatedAt = time.Now()
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	// Serialize event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Publish to Redis
	err = r.client.Publish(ctx, event.Topic, eventData).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	return nil
}

// Subscribe registers a handler for a topic
func (r *RedisEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return r.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (r *RedisEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return r.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (r *RedisEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !r.isStarted {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Create Redis subscription
	var pubsub *redis.PubSub
	if strings.Contains(topic, "*") {
		// Use pattern subscription for wildcard topics
		pubsub = r.client.PSubscribe(ctx, topic)
	} else {
		// Use regular subscription for exact topics
		pubsub = r.client.Subscribe(ctx, topic)
	}

	// Create subscription object
	sub := &redisSubscription{
		id:        uuid.New().String(),
		topic:     topic,
		handler:   handler,
		isAsync:   isAsync,
		pubsub:    pubsub,
		done:      make(chan struct{}),
		cancelled: false,
		bus:       r,
	}

	// Add to subscriptions map
	r.topicMutex.Lock()
	if _, ok := r.subscriptions[topic]; !ok {
		r.subscriptions[topic] = make(map[string]*redisSubscription)
	}
	r.subscriptions[topic][sub.id] = sub
	r.topicMutex.Unlock()

	// Start message listener goroutine
	r.wg.Add(1)
	go r.handleMessages(sub)

	return sub, nil
}

// Unsubscribe removes a subscription
func (r *RedisEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !r.isStarted {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*redisSubscription)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	// Cancel the subscription
	err := sub.Cancel()
	if err != nil {
		return err
	}

	// Remove from subscriptions map
	r.topicMutex.Lock()
	defer r.topicMutex.Unlock()

	if subs, ok := r.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(r.subscriptions, sub.topic)
		}
	}

	return nil
}

// Topics returns a list of all active topics
func (r *RedisEventBus) Topics() []string {
	r.topicMutex.RLock()
	defer r.topicMutex.RUnlock()

	topics := make([]string, 0, len(r.subscriptions))
	for topic := range r.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (r *RedisEventBus) SubscriberCount(topic string) int {
	r.topicMutex.RLock()
	defer r.topicMutex.RUnlock()

	if subs, ok := r.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// handleMessages processes messages for a Redis subscription
func (r *RedisEventBus) handleMessages(sub *redisSubscription) {
	defer r.wg.Done()

	ch := sub.pubsub.Channel()

	for {
		select {
		case <-r.ctx.Done():
			// Event bus is shutting down
			return
		case <-sub.done:
			// Subscription was cancelled
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Deserialize event
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				slog.Error("Failed to deserialize Redis message", "error", err, "topic", msg.Channel)
				continue
			}

			// Process the event
			if sub.isAsync {
				// For async subscriptions, process in a separate goroutine
				go r.processEventAsync(sub, event)
			} else {
				// For sync subscriptions, process immediately
				r.processEvent(sub, event)
			}
		}
	}
}

// processEvent processes an event synchronously
func (r *RedisEventBus) processEvent(sub *redisSubscription, event Event) {
	now := time.Now()
	event.ProcessingStarted = &now

	// Process the event
	err := sub.handler(r.ctx, event)

	// Record completion
	completed := time.Now()
	event.ProcessingCompleted = &completed

	if err != nil {
		// Log error but continue processing
		slog.Error("Redis event handler failed", "error", err, "topic", event.Topic)
	}
}

// processEventAsync processes an event asynchronously
func (r *RedisEventBus) processEventAsync(sub *redisSubscription, event Event) {
	r.processEvent(sub, event)
}
