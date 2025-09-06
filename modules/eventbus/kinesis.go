package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/google/uuid"
)

// Static errors for Kinesis
var (
	ErrInvalidShardCount = errors.New("invalid shard count")
)

// KinesisEventBus implements EventBus using AWS Kinesis
type KinesisEventBus struct {
	config        *KinesisConfig
	client        *kinesis.Client
	subscriptions map[string]map[string]*kinesisSubscription
	topicMutex    sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	isStarted     bool
}

// KinesisConfig holds Kinesis-specific configuration
type KinesisConfig struct {
	Region          string `json:"region"`
	StreamName      string `json:"streamName"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	ShardCount      int32  `json:"shardCount"`
}

// kinesisSubscription represents a subscription in the Kinesis event bus
type kinesisSubscription struct {
	id        string
	topic     string
	handler   EventHandler
	isAsync   bool
	done      chan struct{}
	cancelled bool
	mutex     sync.RWMutex
	bus       *KinesisEventBus
}

// Topic returns the topic of the subscription
func (s *kinesisSubscription) Topic() string {
	return s.topic
}

// ID returns the unique identifier for the subscription
func (s *kinesisSubscription) ID() string {
	return s.id
}

// IsAsync returns whether the subscription is asynchronous
func (s *kinesisSubscription) IsAsync() bool {
	return s.isAsync
}

// Cancel cancels the subscription
func (s *kinesisSubscription) Cancel() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelled {
		return nil
	}

	s.cancelled = true
	close(s.done)
	return nil
}

// NewKinesisEventBus creates a new Kinesis-based event bus
func NewKinesisEventBus(config map[string]interface{}) (EventBus, error) {
	kinesisConfig := &KinesisConfig{
		Region:     "us-east-1",
		StreamName: "eventbus",
		ShardCount: 1,
	}

	// Parse configuration
	if region, ok := config["region"].(string); ok {
		kinesisConfig.Region = region
	}
	if streamName, ok := config["streamName"].(string); ok {
		kinesisConfig.StreamName = streamName
	}
	if accessKeyID, ok := config["accessKeyId"].(string); ok {
		kinesisConfig.AccessKeyID = accessKeyID
	}
	if secretAccessKey, ok := config["secretAccessKey"].(string); ok {
		kinesisConfig.SecretAccessKey = secretAccessKey
	}
	if sessionToken, ok := config["sessionToken"].(string); ok {
		kinesisConfig.SessionToken = sessionToken
	}
	if shardCount, ok := config["shardCount"].(int); ok {
		if shardCount < 1 || shardCount > 2147483647 {
			return nil, fmt.Errorf("%w: shard count out of valid range (1-2147483647): %d", ErrInvalidShardCount, shardCount)
		}
		kinesisConfig.ShardCount = int32(shardCount)
	}

	// Create AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(kinesisConfig.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Kinesis client
	client := kinesis.NewFromConfig(cfg)

	return &KinesisEventBus{
		config:        kinesisConfig,
		client:        client,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
	}, nil
}

// Start initializes the Kinesis event bus
func (k *KinesisEventBus) Start(ctx context.Context) error {
	if k.isStarted {
		return nil
	}

	// Check if stream exists, create if not
	_, err := k.client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: &k.config.StreamName,
	})
	if err != nil {
		// Stream doesn't exist, create it
		// Check for valid shard count
		if k.config.ShardCount < 1 {
			return fmt.Errorf("%w: shard count must be positive: %d", ErrInvalidShardCount, k.config.ShardCount)
		}

		_, err := k.client.CreateStream(ctx, &kinesis.CreateStreamInput{
			StreamName: &k.config.StreamName,
			ShardCount: &k.config.ShardCount,
		})
		if err != nil {
			return fmt.Errorf("failed to create Kinesis stream: %w", err)
		}

		// Wait for stream to become active
		waiter := kinesis.NewStreamExistsWaiter(k.client)
		err = waiter.Wait(ctx, &kinesis.DescribeStreamInput{
			StreamName: &k.config.StreamName,
		}, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to wait for stream to become active: %w", err)
		}
	}

	k.ctx, k.cancel = context.WithCancel(ctx)
	k.isStarted = true
	return nil
}

// Stop shuts down the Kinesis event bus
func (k *KinesisEventBus) Stop(ctx context.Context) error {
	if !k.isStarted {
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
			_ = sub.Cancel() // Ignore error during shutdown
		}
	}
	k.subscriptions = make(map[string]map[string]*kinesisSubscription)
	k.topicMutex.Unlock()

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		k.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers exited gracefully
	case <-ctx.Done():
		return ErrEventBusShutdownTimeout
	}

	k.isStarted = false
	return nil
}

// Publish sends an event to the specified topic using Kinesis
func (k *KinesisEventBus) Publish(ctx context.Context, event Event) error {
	if !k.isStarted {
		return ErrEventBusNotStarted
	}

	// Fill in event metadata
	event.CreatedAt = time.Now()
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	// Add topic to metadata for filtering
	event.Metadata["__topic"] = event.Topic

	// Serialize event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Create Kinesis record
	_, err = k.client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   &k.config.StreamName,
		Data:         eventData,
		PartitionKey: &event.Topic, // Use topic as partition key
	})
	if err != nil {
		return fmt.Errorf("failed to publish to Kinesis: %w", err)
	}

	return nil
}

// Subscribe registers a handler for a topic
func (k *KinesisEventBus) Subscribe(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return k.subscribe(ctx, topic, handler, false)
}

// SubscribeAsync registers a handler for a topic with asynchronous processing
func (k *KinesisEventBus) SubscribeAsync(ctx context.Context, topic string, handler EventHandler) (Subscription, error) {
	return k.subscribe(ctx, topic, handler, true)
}

// subscribe is the internal implementation for both Subscribe and SubscribeAsync
func (k *KinesisEventBus) subscribe(ctx context.Context, topic string, handler EventHandler, isAsync bool) (Subscription, error) {
	if !k.isStarted {
		return nil, ErrEventBusNotStarted
	}

	if handler == nil {
		return nil, ErrEventHandlerNil
	}

	// Create subscription object
	sub := &kinesisSubscription{
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
		k.subscriptions[topic] = make(map[string]*kinesisSubscription)
	}
	k.subscriptions[topic][sub.id] = sub
	k.topicMutex.Unlock()

	// Start shard reader if this is the first subscription
	k.startShardReaders()

	return sub, nil
}

// startShardReaders starts reading from all shards
func (k *KinesisEventBus) startShardReaders() {
	// Get stream description to find shards
	// sync.WaitGroup.Go used (Go >=1.23); improves correctness by tying Add/Done
	// to function scope. Legacy pattern would manually Add(1)/defer Done().
	k.wg.Go(func() {
		for {
			select {
			case <-k.ctx.Done():
				return
			default:
				// List shards
				resp, err := k.client.DescribeStream(k.ctx, &kinesis.DescribeStreamInput{
					StreamName: &k.config.StreamName,
				})
				if err != nil {
					slog.Error("Failed to describe Kinesis stream", "error", err)
					time.Sleep(5 * time.Second)
					continue
				}

				// Start reader for each shard
				for _, shard := range resp.StreamDescription.Shards {
					go k.readShard(*shard.ShardId)
				}

				// Sleep before checking for new shards
				time.Sleep(30 * time.Second)
			}
		}
	})
}

// readShard reads records from a specific shard
func (k *KinesisEventBus) readShard(shardID string) {
	k.wg.Add(1)
	defer k.wg.Done()

	// Get shard iterator
	iterResp, err := k.client.GetShardIterator(k.ctx, &kinesis.GetShardIteratorInput{
		StreamName:        &k.config.StreamName,
		ShardId:           &shardID,
		ShardIteratorType: types.ShardIteratorTypeLatest,
	})
	if err != nil {
		slog.Error("Failed to get Kinesis shard iterator", "error", err, "shard", shardID)
		return
	}

	shardIterator := iterResp.ShardIterator

	for {
		select {
		case <-k.ctx.Done():
			return
		default:
			if shardIterator == nil {
				return
			}

			// Get records
			resp, err := k.client.GetRecords(k.ctx, &kinesis.GetRecordsInput{
				ShardIterator: shardIterator,
			})
			if err != nil {
				slog.Error("Failed to get Kinesis records", "error", err, "shard", shardID)
				time.Sleep(1 * time.Second)
				continue
			}

			// Process records
			for _, record := range resp.Records {
				var event Event
				if err := json.Unmarshal(record.Data, &event); err != nil {
					slog.Error("Failed to deserialize Kinesis record", "error", err)
					continue
				}

				// Find matching subscriptions
				k.topicMutex.RLock()
				subs := make([]*kinesisSubscription, 0)
				for _, subsMap := range k.subscriptions {
					for _, sub := range subsMap {
						if k.topicMatches(event.Topic, sub.topic) {
							subs = append(subs, sub)
						}
					}
				}
				k.topicMutex.RUnlock()

				// Process event for each matching subscription
				for _, sub := range subs {
					if sub.isAsync {
						go k.processEventAsync(sub, event)
					} else {
						k.processEvent(sub, event)
					}
				}
			}

			// Update shard iterator
			shardIterator = resp.NextShardIterator

			// Sleep to avoid hitting API limits
			time.Sleep(1 * time.Second)
		}
	}
}

// topicMatches checks if a topic matches a subscription pattern
func (k *KinesisEventBus) topicMatches(eventTopic, subscriptionTopic string) bool {
	if eventTopic == subscriptionTopic {
		return true
	}

	if strings.HasSuffix(subscriptionTopic, "*") {
		prefix := subscriptionTopic[:len(subscriptionTopic)-1]
		return strings.HasPrefix(eventTopic, prefix)
	}

	return false
}

// Unsubscribe removes a subscription
func (k *KinesisEventBus) Unsubscribe(ctx context.Context, subscription Subscription) error {
	if !k.isStarted {
		return ErrEventBusNotStarted
	}

	sub, ok := subscription.(*kinesisSubscription)
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
func (k *KinesisEventBus) Topics() []string {
	k.topicMutex.RLock()
	defer k.topicMutex.RUnlock()

	topics := make([]string, 0, len(k.subscriptions))
	for topic := range k.subscriptions {
		topics = append(topics, topic)
	}

	return topics
}

// SubscriberCount returns the number of subscribers for a topic
func (k *KinesisEventBus) SubscriberCount(topic string) int {
	k.topicMutex.RLock()
	defer k.topicMutex.RUnlock()

	if subs, ok := k.subscriptions[topic]; ok {
		return len(subs)
	}

	return 0
}

// processEvent processes an event synchronously
func (k *KinesisEventBus) processEvent(sub *kinesisSubscription, event Event) {
	now := time.Now()
	event.ProcessingStarted = &now

	// Process the event
	err := sub.handler(k.ctx, event)

	// Record completion
	completed := time.Now()
	event.ProcessingCompleted = &completed

	if err != nil {
		// Log error but continue processing
		slog.Error("Kinesis event handler failed", "error", err, "topic", event.Topic)
	}
}

// processEventAsync processes an event asynchronously
func (k *KinesisEventBus) processEventAsync(sub *kinesisSubscription, event Event) {
	k.processEvent(sub, event)
}
