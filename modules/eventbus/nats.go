package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NatsEventBus implements EventBus using NATS messaging
type NatsEventBus struct {
	config        *NatsConfig
	conn          *nats.Conn
	subscriptions map[string]map[string]*natsSubscription
	topicMutex    sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	isStarted     atomic.Bool
}

// NatsConfig holds NATS-specific configuration
type NatsConfig struct {
	URL              string `json:"url"`
	Username         string `json:"username"`
	Password         string `json:"password"` //nolint:gosec // config field, not a hardcoded secret
	Token            string `json:"token"`
	MaxReconnects    int    `json:"maxReconnects"`
	ReconnectWait    int    `json:"reconnectWait"`
	ConnectionName   string `json:"connectionName"`
	AllowReconnect   bool   `json:"allowReconnect"`
	PingInterval     int    `json:"pingInterval"`
	MaxPingsOut      int    `json:"maxPingsOut"`
	SubscribeTimeout int    `json:"subscribeTimeout"`
}

// natsSubscription represents a subscription in the NATS event bus
type natsSubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	natsSub   *nats.Subscription
	done      chan struct{}
	cancelled bool
	mutex     sync.RWMutex
	bus       *NatsEventBus
}

// Topic returns the topic of the subscription
func (s *natsSubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *natsSubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *natsSubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *natsSubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	s.cancelled = true
	if s.natsSub != nil {
		_ = s.natsSub.Unsubscribe()
	}
	close(s.done)
	return nil
}

// NewNatsEventBus creates a new NATS-based event bus
func NewNatsEventBus(config map[string]interface{}) (EventBus, error) {
	natsConfig := &NatsConfig{
		URL:              nats.DefaultURL,
		MaxReconnects:    10,
		ReconnectWait:    2,
		AllowReconnect:   true,
		PingInterval:     20,
		MaxPingsOut:      2,
		SubscribeTimeout: 5,
		ConnectionName:   "modular-eventbus",
	}

	// Parse configuration
	if url, ok := config["url"].(string); ok {
		natsConfig.URL = url
	}
	if username, ok := config["username"].(string); ok {
		natsConfig.Username = username
	}
	if password, ok := config["password"].(string); ok {
		natsConfig.Password = password
	}
	if token, ok := config["token"].(string); ok {
		natsConfig.Token = token
	}
	if maxReconnects, ok := config["maxReconnects"].(int); ok {
		natsConfig.MaxReconnects = maxReconnects
	}
	if reconnectWait, ok := config["reconnectWait"].(int); ok {
		natsConfig.ReconnectWait = reconnectWait
	}
	if connName, ok := config["connectionName"].(string); ok {
		natsConfig.ConnectionName = connName
	}
	if allowReconnect, ok := config["allowReconnect"].(bool); ok {
		natsConfig.AllowReconnect = allowReconnect
	}
	if pingInterval, ok := config["pingInterval"].(int); ok {
		natsConfig.PingInterval = pingInterval
	}
	if maxPingsOut, ok := config["maxPingsOut"].(int); ok {
		natsConfig.MaxPingsOut = maxPingsOut
	}
	if subscribeTimeout, ok := config["subscribeTimeout"].(int); ok {
		natsConfig.SubscribeTimeout = subscribeTimeout
	}

	// Create NATS connection options
	opts := []nats.Option{
		nats.Name(natsConfig.ConnectionName),
		nats.MaxReconnects(natsConfig.MaxReconnects),
		nats.ReconnectWait(time.Duration(natsConfig.ReconnectWait) * time.Second),
		nats.PingInterval(time.Duration(natsConfig.PingInterval) * time.Second),
		nats.MaxPingsOutstanding(natsConfig.MaxPingsOut),
	}

	if !natsConfig.AllowReconnect {
		opts = append(opts, nats.NoReconnect())
	}

	// Add authentication if provided
	if natsConfig.Token != "" {
		opts = append(opts, nats.Token(natsConfig.Token))
	} else if natsConfig.Username != "" && natsConfig.Password != "" {
		opts = append(opts, nats.UserInfo(natsConfig.Username, natsConfig.Password))
	}

	// Connect to NATS
	conn, err := nats.Connect(natsConfig.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NatsEventBus{
		config:        natsConfig,
		conn:          conn,
		subscriptions: make(map[string]map[string]*natsSubscription),
	}, nil
}

// Start initializes the NATS event bus
func (n *NatsEventBus) Start(ctx context.Context) error {
	if n.isStarted.Load() {
		return nil
	}

	// Check if connection is valid
	if n.conn.Status() != nats.CONNECTED {
		return ErrNATSConnectionNotEstablished
	}

	n.ctx, n.cancel = context.WithCancel(ctx) //nolint:gosec // G118: cancel is stored in n.cancel and called in Stop()
	n.isStarted.Store(true)
	return nil
}

// Stop shuts down the NATS event bus
func (n *NatsEventBus) Stop(ctx context.Context) error {
	if !n.isStarted.Load() {
		return nil
	}

	// Cancel context to signal all workers to stop
	if n.cancel != nil {
		n.cancel()
	}

	// Cancel all subscriptions
	n.topicMutex.Lock()
	for _, subs := range n.subscriptions {
		for _, sub := range subs {
			_ = sub.Cancel() // Ignore error during shutdown
		}
	}
	n.subscriptions = make(map[string]map[string]*natsSubscription)
	n.topicMutex.Unlock()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers exited gracefully
	case <-ctx.Done():
		return ErrEventBusShutdownTimeout
	}

	// Close NATS connection
	n.conn.Close()

	n.isStarted.Store(false)
	return nil
}

// Publish sends an event to the specified topic using NATS
func (n *NatsEventBus) Publish(ctx context.Context, event Event) error {
	if !n.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Convert topic to NATS subject (replace wildcards if needed)
	subject := n.topicToSubject(event.Type())

	// Publish to NATS
	err = n.conn.Publish(subject, eventData)
	if err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	return nil
}

// Subscribe registers a handler for a topic
func (n *NatsEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return n.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (n *NatsEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return n.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (n *NatsEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !n.isStarted.Load() {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Convert topic to NATS subject
	subject := n.topicToSubject(topic)

	// Create subscription object
	sub := &natsSubscription{
		id:        uuid.New().String(),
		topic:     topic,
		handler:   handler,
		isAsync:   isAsync,
		done:      make(chan struct{}),
		cancelled: false,
		bus:       n,
	}

	// Create NATS subscription with message handler
	natsSub, err := n.conn.Subscribe(subject, func(msg *nats.Msg) {
		// Check if subscription is cancelled
		sub.mutex.RLock()
		if sub.cancelled {
			sub.mutex.RUnlock()
			return
		}
		sub.mutex.RUnlock()

		// Deserialize event
		var event Event
		err := json.Unmarshal(msg.Data, &event)
		if err != nil {
			slog.Error("Failed to deserialize NATS message", "error", err, "subject", msg.Subject)
			return
		}

		// Process the event
		if sub.isAsync {
			// For async subscriptions, process in a separate goroutine
			n.wg.Add(1)
			go func() {
				defer n.wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("nats subscription panic", "error", r)
					}
				}()
				n.processEvent(sub, event)
			}()
		} else {
			// For sync subscriptions, process immediately
			n.processEvent(sub, event)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to NATS: %w", err)
	}

	sub.natsSub = natsSub

	// Add to subscriptions map
	n.topicMutex.Lock()
	if _, ok := n.subscriptions[topic]; !ok {
		n.subscriptions[topic] = make(map[string]*natsSubscription)
	}
	n.subscriptions[topic][sub.id] = sub
	n.topicMutex.Unlock()

	return sub, nil
}

// Unsubscribe removes a subscription
func (n *NatsEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !n.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*natsSubscription)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	// Cancel the subscription
	err := sub.Cancel()
	if err != nil {
		return err
	}

	// Remove from subscriptions map
	n.topicMutex.Lock()
	defer n.topicMutex.Unlock()

	if subs, ok := n.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(n.subscriptions, sub.topic)
		}
	}

	return nil
}

// Topics returns a list of all active topics
func (n *NatsEventBus) Topics() []string {
	n.topicMutex.RLock()
	defer n.topicMutex.RUnlock()

	topics := make([]string, 0, len(n.subscriptions))
	for topic := range n.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (n *NatsEventBus) SubscriberCount(topic string) int {
	n.topicMutex.RLock()
	defer n.topicMutex.RUnlock()

	if subs, ok := n.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// processEvent processes an event synchronously
func (n *NatsEventBus) processEvent(sub *natsSubscription, event Event) {
	err := sub.handler(n.ctx, event)
	if err != nil {
		slog.Error("NATS event handler failed", "error", err, "topic", event.Type())
	}
}

// topicToSubject converts an eventbus topic pattern to a NATS subject
// EventBus uses "user.*" style wildcards, NATS uses "user.>" for multi-level wildcards
func (n *NatsEventBus) topicToSubject(topic string) string {
	// Replace trailing wildcard with NATS multi-level wildcard
	if strings.HasSuffix(topic, ".*") {
		return strings.TrimSuffix(topic, "*") + ">"
	}
	if topic == "*" {
		return ">"
	}
	return topic
}
