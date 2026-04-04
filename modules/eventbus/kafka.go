package eventbus

//go:generate mockgen -destination=mocks/mock_kafka.go -package=mocks github.com/IBM/sarama SyncProducer,ConsumerGroup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

// KafkaEventBus implements EventBus using Apache Kafka
type KafkaEventBus struct {
	config          *KafkaConfig
	producer        sarama.SyncProducer
	consumerGroup   sarama.ConsumerGroup
	subscriptions   map[string]map[string]*kafkaSubscription
	topicMutex      sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	isStarted       atomic.Bool
	consumerGroupID string
}

// KafkaConfig holds Kafka-specific configuration
type KafkaConfig struct {
	Brokers        []string          `json:"brokers"`
	GroupID        string            `json:"groupId"`
	SecurityConfig map[string]string `json:"security"`
	ProducerConfig map[string]string `json:"producer"`
	ConsumerConfig map[string]string `json:"consumer"`
}

// kafkaSubscription represents a subscription in the Kafka event bus
type kafkaSubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	done      chan struct{}
	cancelled bool
	mutex     sync.RWMutex
	bus       *KafkaEventBus
}

// Topic returns the topic of the subscription
func (s *kafkaSubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *kafkaSubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *kafkaSubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *kafkaSubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	s.cancelled = true
	close(s.done)
	return nil
}

// KafkaConsumerGroupHandler implements sarama.ConsumerGroupHandler
type KafkaConsumerGroupHandler struct {
	eventBus      *KafkaEventBus
	subscriptions map[string]*kafkaSubscription
	mutex         sync.RWMutex
}

// Setup is called at the beginning of a new session, before ConsumeClaim
func (h *KafkaConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called at the end of a session, once all ConsumeClaim goroutines have exited
func (h *KafkaConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a Kafka partition
func (h *KafkaConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case msg := <-claim.Messages():
			if msg == nil {
				return nil
			}

			// Find subscriptions for this topic
			h.mutex.RLock()
			subs := make([]*kafkaSubscription, 0)
			for _, sub := range h.subscriptions {
				if h.topicMatches(msg.Topic, sub.topic) {
					subs = append(subs, sub)
				}
			}
			h.mutex.RUnlock()

			// Deserialize once per message, reuse for all matching subscriptions
			var event Event
			err := json.Unmarshal(msg.Value, &event)
			if err != nil {
				slog.Error("Failed to deserialize Kafka message", "error", err, "topic", msg.Topic)
				session.MarkMessage(msg, "")
				continue
			}

			// Process message for each matching subscription
			for _, sub := range subs {
				if sub.isAsync {
					go h.eventBus.processEventAsync(sub, event)
				} else {
					h.eventBus.processEvent(sub, event)
				}
			}

			// Mark message as processed
			session.MarkMessage(msg, "")
		}
	}
}

// topicMatches checks if a topic matches a subscription pattern
func (h *KafkaConsumerGroupHandler) topicMatches(messageTopic, subscriptionTopic string) bool {
	if messageTopic == subscriptionTopic {
		return true
	}

	if strings.HasSuffix(subscriptionTopic, "*") {
		prefix := subscriptionTopic[:len(subscriptionTopic)-1]
		return strings.HasPrefix(messageTopic, prefix)
	}

	return false
}

// NewKafkaEventBus creates a new Kafka-based event bus
func NewKafkaEventBus(config map[string]interface{}) (EventBus, error) {
	kafkaConfig := &KafkaConfig{
		Brokers:        []string{"localhost:9092"},
		GroupID:        "eventbus-" + uuid.New().String(),
		SecurityConfig: make(map[string]string),
		ProducerConfig: make(map[string]string),
		ConsumerConfig: make(map[string]string),
	}

	// Parse configuration
	if brokers, ok := config["brokers"].([]interface{}); ok {
		kafkaConfig.Brokers = make([]string, len(brokers))
		for i, broker := range brokers {
			kafkaConfig.Brokers[i] = broker.(string)
		}
	}
	if groupID, ok := config["groupId"].(string); ok {
		kafkaConfig.GroupID = groupID
	}
	if security, ok := config["security"].(map[string]interface{}); ok {
		for k, v := range security {
			kafkaConfig.SecurityConfig[k] = v.(string)
		}
	}

	// Create Sarama configuration
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_6_0_0
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	// Apply security configuration
	for key, value := range kafkaConfig.SecurityConfig {
		switch key {
		case "sasl.mechanism":
			if value == "PLAIN" {
				saramaConfig.Net.SASL.Enable = true
				saramaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
			}
		case "sasl.username":
			saramaConfig.Net.SASL.User = value
		case "sasl.password":
			saramaConfig.Net.SASL.Password = value
		case "security.protocol":
			if value == "SSL" {
				saramaConfig.Net.TLS.Enable = true
			}
		}
	}

	// Create producer
	producer, err := sarama.NewSyncProducer(kafkaConfig.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(kafkaConfig.Brokers, kafkaConfig.GroupID, saramaConfig)
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("failed to create Kafka consumer group: %w", err)
	}

	return &KafkaEventBus{
		config:          kafkaConfig,
		producer:        producer,
		consumerGroup:   consumerGroup,
		subscriptions:   make(map[string]map[string]*kafkaSubscription),
		consumerGroupID: kafkaConfig.GroupID,
	}, nil
}

// Start initializes the Kafka event bus
func (k *KafkaEventBus) Start(ctx context.Context) error {
	if k.isStarted.Load() {
		return nil
	}

	k.ctx, k.cancel = context.WithCancel(ctx) //nolint:gosec // G118: cancel is stored in k.cancel and called in Stop()
	k.isStarted.Store(true)
	return nil
}

// Stop shuts down the Kafka event bus
func (k *KafkaEventBus) Stop(ctx context.Context) error {
	if !k.isStarted.Load() {
		return nil
	}

	// Cancel context to signal all workers to stop
	if k.cancel != nil {
		k.cancel()
	}

	// Cancel all subscriptions
	k.topicMutex.Lock()
	for _, subs := range k.subscriptions {
		for _, sub := range subs {
			if err := sub.Cancel(); err != nil {
				slog.Warn("failed to cancel Kafka subscription during shutdown", "error", err)
			}
		}
	}
	k.subscriptions = make(map[string]map[string]*kafkaSubscription)
	k.topicMutex.Unlock()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in Kafka eventbus shutdown waiter", "error", r)
			}
		}()
		k.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers exited gracefully
	case <-ctx.Done():
		return ErrEventBusShutdownTimeout
	}

	// Close Kafka connections
	if err := k.producer.Close(); err != nil {
		return fmt.Errorf("error closing Kafka producer: %w", err)
	}
	if err := k.consumerGroup.Close(); err != nil {
		return fmt.Errorf("error closing Kafka consumer group: %w", err)
	}

	k.isStarted.Store(false)
	return nil
}

// Publish sends an event to the specified topic using Kafka
func (k *KafkaEventBus) Publish(ctx context.Context, event Event) error {
	if !k.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Create Kafka message
	message := &sarama.ProducerMessage{
		Topic: event.Type(),
		Value: sarama.StringEncoder(eventData),
	}

	// Set partition key if provided (otherwise uses client's default partitioner)
	if key, ok := PartitionKeyFromContext(ctx); ok {
		message.Key = sarama.StringEncoder(key)
	}

	// Publish to Kafka
	_, _, err = k.producer.SendMessage(message)
	if err != nil {
		return fmt.Errorf("failed to publish to Kafka: %w", err)
	}

	return nil
}

// Subscribe registers a handler for a topic
func (k *KafkaEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return k.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (k *KafkaEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return k.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (k *KafkaEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !k.isStarted.Load() {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Create subscription object
	sub := &kafkaSubscription{
		id:        uuid.New().String(),
		topic:     topic,
		handler:   handler,
		isAsync:   isAsync,
		done:      make(chan struct{}),
		cancelled: false,
		bus:       k,
	}

	// Add to subscriptions map
	k.topicMutex.Lock()
	if _, ok := k.subscriptions[topic]; !ok {
		k.subscriptions[topic] = make(map[string]*kafkaSubscription)
	}
	k.subscriptions[topic][sub.id] = sub
	k.topicMutex.Unlock()

	// Start consumer group for this topic if not already started
	go k.startConsumerGroup()

	return sub, nil
}

// startConsumerGroup starts the Kafka consumer group
func (k *KafkaEventBus) startConsumerGroup() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in Kafka consumer group", "error", r)
		}
	}()
	handler := &KafkaConsumerGroupHandler{
		eventBus:      k,
		subscriptions: make(map[string]*kafkaSubscription),
	}

	// Collect all subscriptions
	k.topicMutex.RLock()
	topics := make([]string, 0)
	for topic, subs := range k.subscriptions {
		topics = append(topics, topic)
		for _, sub := range subs {
			handler.subscriptions[sub.id] = sub
		}
	}
	k.topicMutex.RUnlock()

	if len(topics) == 0 {
		return
	}

	// Start consuming (Go 1.25 WaitGroup.Go)
	k.wg.Go(func() {
		for {
			if err := k.consumerGroup.Consume(k.ctx, topics, handler); err != nil {
				slog.Error("Kafka consumer group error", "error", err)
			}
			if k.ctx.Err() != nil {
				return
			}
		}
	})
}

// Unsubscribe removes a subscription
func (k *KafkaEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !k.isStarted.Load() {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*kafkaSubscription)
	if !ok {
		return ErrInvalidSubscriptionType
	}

	// Cancel the subscription
	err := sub.Cancel()
	if err != nil {
		return err
	}

	// Remove from subscriptions map
	k.topicMutex.Lock()
	defer k.topicMutex.Unlock()

	if subs, ok := k.subscriptions[sub.topic]; ok {
		delete(subs, sub.id)
		if len(subs) == 0 {
			delete(k.subscriptions, sub.topic)
		}
	}

	return nil
}

// Topics returns a list of all active topics
func (k *KafkaEventBus) Topics() []string {
	k.topicMutex.RLock()
	defer k.topicMutex.RUnlock()

	topics := make([]string, 0, len(k.subscriptions))
	for topic := range k.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (k *KafkaEventBus) SubscriberCount(topic string) int {
	k.topicMutex.RLock()
	defer k.topicMutex.RUnlock()

	if subs, ok := k.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// processEvent processes an event synchronously
func (k *KafkaEventBus) processEvent(sub *kafkaSubscription, event Event) {
	err := sub.handler(k.ctx, event)
	if err != nil {
		slog.Error("Kafka event handler failed", "error", err, "topic", event.Type())
	}
}

// processEventAsync processes an event asynchronously
func (k *KafkaEventBus) processEventAsync(sub *kafkaSubscription, event Event) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in Kafka async event handler", "error", r, "topic", event.Type())
		}
	}()
	k.processEvent(sub, event)
}
