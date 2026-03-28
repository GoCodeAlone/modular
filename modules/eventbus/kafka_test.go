package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/GoCodeAlone/modular/modules/eventbus/v2/mocks"
)

// newTestKafkaEventBus creates a KafkaEventBus wired to a mock producer,
// pre-started so Publish() can be called immediately.
func newTestKafkaEventBus(producer sarama.SyncProducer) *KafkaEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	bus := &KafkaEventBus{
		config: &KafkaConfig{
			Brokers: []string{"localhost:9092"},
			GroupID: "test-group",
		},
		producer:      producer,
		subscriptions: make(map[string]map[string]*kafkaSubscription),
		ctx:           ctx,
		cancel:        cancel,
	}
	bus.isStarted.Store(true)
	return bus
}

func TestKafkaPublishPartitionKey(t *testing.T) {
	t.Run("sets message key from context partition key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			SendMessage(gomock.Any()).
			DoAndReturn(func(msg *sarama.ProducerMessage) (int32, int64, error) {
				assert.Equal(t, "orders.created", msg.Topic)
				keyBytes, err := msg.Key.Encode()
				require.NoError(t, err)
				assert.Equal(t, "user-42", string(keyBytes))
				return 0, 0, nil
			})

		ctx := WithPartitionKey(context.Background(), "user-42")
		err := bus.Publish(ctx, newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("message key is nil when no partition key in context", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			SendMessage(gomock.Any()).
			DoAndReturn(func(msg *sarama.ProducerMessage) (int32, int64, error) {
				assert.Nil(t, msg.Key, "message key should be nil when no partition key set")
				return 0, 0, nil
			})

		err := bus.Publish(context.Background(), newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("empty string partition key is honored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			SendMessage(gomock.Any()).
			DoAndReturn(func(msg *sarama.ProducerMessage) (int32, int64, error) {
				require.NotNil(t, msg.Key, "empty string should be honored as a key for Kafka")
				keyBytes, err := msg.Key.Encode()
				require.NoError(t, err)
				assert.Equal(t, "", string(keyBytes))
				return 0, 0, nil
			})

		ctx := WithPartitionKey(context.Background(), "")
		err := bus.Publish(ctx, newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("propagates SendMessage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			SendMessage(gomock.Any()).
			Return(int32(0), int64(0), fmt.Errorf("broker unavailable"))

		err := bus.Publish(context.Background(), newTestCloudEvent("test", "data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "broker unavailable")
	})
}

func TestKafkaPublishCloudEventsWireFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockSyncProducer(ctrl)
	bus := newTestKafkaEventBus(m)
	defer bus.cancel()

	m.EXPECT().
		SendMessage(gomock.Any()).
		DoAndReturn(func(msg *sarama.ProducerMessage) (int32, int64, error) {
			valueBytes, err := msg.Value.Encode()
			require.NoError(t, err)

			var wire map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(valueBytes, &wire))

			assert.Contains(t, wire, "specversion", "wire format should have specversion at top level")
			assert.Contains(t, wire, "type")
			assert.Contains(t, wire, "source")
			assert.NotContains(t, wire, "topic", "wire format should NOT have Event.Topic wrapper")
			assert.NotContains(t, wire, "payload", "wire format should NOT have Event.Payload wrapper")
			return 0, 0, nil
		})

	e := newTestCloudEvent("messaging.texter-message.received", map[string]interface{}{"messageId": "msg-456"})
	e.SetSource("/chimera/messaging")
	e.SetID("evt-123")

	err := bus.Publish(context.Background(), e)
	require.NoError(t, err)
}

func TestKafkaStart(t *testing.T) {
	t.Run("sets started state", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
		}

		err := bus.Start(context.Background())
		require.NoError(t, err)
		assert.True(t, bus.isStarted.Load())
	})

	t.Run("returns nil when already started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(producer)
		defer bus.cancel()

		err := bus.Start(context.Background())
		require.NoError(t, err)
	})
}

func TestKafkaStop(t *testing.T) {
	t.Run("returns nil when not started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
		}

		err := bus.Stop(context.Background())
		require.NoError(t, err)
	})

	t.Run("closes producer and consumer group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		consumerGroup := mocks.NewMockConsumerGroup(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			consumerGroup: consumerGroup,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
			ctx:           ctx,
			cancel:        cancel,
		}
		bus.isStarted.Store(true)

		producer.EXPECT().Close().Return(nil)
		consumerGroup.EXPECT().Close().Return(nil)

		err := bus.Stop(context.Background())
		require.NoError(t, err)
		assert.False(t, bus.isStarted.Load())
	})

	t.Run("propagates producer close error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		consumerGroup := mocks.NewMockConsumerGroup(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			consumerGroup: consumerGroup,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
			ctx:           ctx,
			cancel:        cancel,
		}
		bus.isStarted.Store(true)

		producer.EXPECT().Close().Return(fmt.Errorf("producer close failed"))

		err := bus.Stop(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "producer close failed")
	})

	t.Run("propagates consumer group close error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		consumerGroup := mocks.NewMockConsumerGroup(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			consumerGroup: consumerGroup,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
			ctx:           ctx,
			cancel:        cancel,
		}
		bus.isStarted.Store(true)

		producer.EXPECT().Close().Return(nil)
		consumerGroup.EXPECT().Close().Return(fmt.Errorf("consumer group close failed"))

		err := bus.Stop(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consumer group close failed")
	})

	t.Run("returns timeout error when context expires", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)

		ctx, cancel := context.WithCancel(context.Background())
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
			ctx:           ctx,
			cancel:        cancel,
		}
		bus.isStarted.Store(true)

		// Add a wait group entry that never completes
		bus.wg.Add(1)

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer stopCancel()

		err := bus.Stop(stopCtx)
		assert.ErrorIs(t, err, ErrEventBusShutdownTimeout)

		bus.wg.Done()
	})
}

func TestKafkaSubscribe(t *testing.T) {
	t.Run("returns error when not started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		bus := &KafkaEventBus{
			config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
			producer:      producer,
			subscriptions: make(map[string]map[string]*kafkaSubscription),
		}

		_, err := bus.Subscribe(context.Background(), "topic", func(ctx context.Context, event Event) error { return nil })
		assert.ErrorIs(t, err, ErrEventBusNotStarted)
	})

	t.Run("returns error for nil handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		producer := mocks.NewMockSyncProducer(ctrl)
		bus := newTestKafkaEventBus(producer)
		defer bus.cancel()

		_, err := bus.Subscribe(context.Background(), "topic", nil)
		assert.ErrorIs(t, err, ErrEventHandlerNil)
	})
}

func TestKafkaPublishNotStarted(t *testing.T) {
	ctrl := gomock.NewController(t)
	producer := mocks.NewMockSyncProducer(ctrl)
	bus := &KafkaEventBus{
		config:        &KafkaConfig{Brokers: []string{"localhost:9092"}, GroupID: "test"},
		producer:      producer,
		subscriptions: make(map[string]map[string]*kafkaSubscription),
	}

	err := bus.Publish(context.Background(), newTestCloudEvent("test", "data"))
	assert.ErrorIs(t, err, ErrEventBusNotStarted)
}

// --- ConsumeClaim integration tests: CloudEvents deserialization via Kafka ---

// testConsumerGroupSession implements sarama.ConsumerGroupSession for tests.
type testConsumerGroupSession struct {
	ctx        context.Context
	markedMsgs []*sarama.ConsumerMessage
}

func (s *testConsumerGroupSession) Claims() map[string][]int32                       { return nil }
func (s *testConsumerGroupSession) MemberID() string                                 { return "test-member" }
func (s *testConsumerGroupSession) GenerationID() int32                              { return 1 }
func (s *testConsumerGroupSession) MarkOffset(_ string, _ int32, _ int64, _ string)  {}
func (s *testConsumerGroupSession) Commit()                                          {}
func (s *testConsumerGroupSession) ResetOffset(_ string, _ int32, _ int64, _ string) {}
func (s *testConsumerGroupSession) Context() context.Context                         { return s.ctx }
func (s *testConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	s.markedMsgs = append(s.markedMsgs, msg)
}

// testConsumerGroupClaim implements sarama.ConsumerGroupClaim for tests.
type testConsumerGroupClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (c *testConsumerGroupClaim) Topic() string                            { return "test-topic" }
func (c *testConsumerGroupClaim) Partition() int32                         { return 0 }
func (c *testConsumerGroupClaim) InitialOffset() int64                     { return 0 }
func (c *testConsumerGroupClaim) HighWaterMarkOffset() int64               { return 0 }
func (c *testConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }

func TestKafkaConsumeClaimCloudEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	producer := mocks.NewMockSyncProducer(ctrl)
	bus := newTestKafkaEventBus(producer)
	defer bus.cancel()

	received := make(chan Event, 1)
	handler := &KafkaConsumerGroupHandler{
		eventBus: bus,
		subscriptions: map[string]*kafkaSubscription{
			"sub-1": {
				id:    "sub-1",
				topic: "order.placed",
				handler: func(ctx context.Context, event Event) error {
					received <- event
					return nil
				},
				done: make(chan struct{}),
				bus:  bus,
			},
		},
	}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic: "order.placed",
		Value: []byte(`{"specversion":"1.0","type":"order.placed","source":"order-svc","id":"evt-1","data":{"orderId":"42"}}`),
	}
	close(messages)

	session := &testConsumerGroupSession{ctx: context.Background()}
	claim := &testConsumerGroupClaim{messages: messages}

	err := handler.ConsumeClaim(session, claim)
	require.NoError(t, err)

	select {
	case event := <-received:
		assert.Equal(t, "order.placed", event.Type())
		assert.Equal(t, "1.0", event.SpecVersion())
		assert.Equal(t, "order-svc", event.Source())
		var payloadMap map[string]interface{}
		require.NoError(t, event.DataAs(&payloadMap))
		assert.Equal(t, "42", payloadMap["orderId"])
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	assert.Len(t, session.markedMsgs, 1)
}

func TestKafkaConsumeClaimRejectsInvalidRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	producer := mocks.NewMockSyncProducer(ctrl)
	bus := newTestKafkaEventBus(producer)
	defer bus.cancel()

	handlerCalled := make(chan struct{}, 1)
	handler := &KafkaConsumerGroupHandler{
		eventBus: bus,
		subscriptions: map[string]*kafkaSubscription{
			"sub-1": {
				id:    "sub-1",
				topic: "order.placed",
				handler: func(ctx context.Context, event Event) error {
					handlerCalled <- struct{}{}
					return nil
				},
				done: make(chan struct{}),
				bus:  bus,
			},
		},
	}

	messages := make(chan *sarama.ConsumerMessage, 1)
	messages <- &sarama.ConsumerMessage{
		Topic: "order.placed",
		Value: []byte(`not valid json`),
	}
	close(messages)

	session := &testConsumerGroupSession{ctx: context.Background()}
	claim := &testConsumerGroupClaim{messages: messages}

	err := handler.ConsumeClaim(session, claim)
	require.NoError(t, err)

	// Handler should NOT have been called.
	select {
	case <-handlerCalled:
		t.Fatal("handler should not have been called for invalid record")
	case <-time.After(100 * time.Millisecond):
		// Success: handler was not called.
	}

	// Message should still be marked (offset committed even on error).
	assert.Len(t, session.markedMsgs, 1)
}

func TestKafkaConsumeClaimMultipleMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	producer := mocks.NewMockSyncProducer(ctrl)
	bus := newTestKafkaEventBus(producer)
	defer bus.cancel()

	orderReceived := make(chan Event, 1)
	userReceived := make(chan Event, 1)
	handler := &KafkaConsumerGroupHandler{
		eventBus: bus,
		subscriptions: map[string]*kafkaSubscription{
			"order-sub": {
				id:    "order-sub",
				topic: "order.placed",
				handler: func(ctx context.Context, e Event) error {
					orderReceived <- e
					return nil
				},
				done: make(chan struct{}),
				bus:  bus,
			},
			"user-sub": {
				id:    "user-sub",
				topic: "user.created",
				handler: func(ctx context.Context, e Event) error {
					userReceived <- e
					return nil
				},
				done: make(chan struct{}),
				bus:  bus,
			},
		},
	}

	messages := make(chan *sarama.ConsumerMessage, 2)
	messages <- &sarama.ConsumerMessage{
		Topic: "order.placed",
		Value: []byte(`{"specversion":"1.0","type":"order.placed","source":"orders","id":"1","data":{"orderId":"o1"}}`),
	}
	messages <- &sarama.ConsumerMessage{
		Topic: "user.created",
		Value: []byte(`{"specversion":"1.0","type":"user.created","source":"users","id":"2","data":{"userId":"u1"}}`),
	}
	close(messages)

	session := &testConsumerGroupSession{ctx: context.Background()}
	claim := &testConsumerGroupClaim{messages: messages}

	err := handler.ConsumeClaim(session, claim)
	require.NoError(t, err)

	select {
	case e := <-orderReceived:
		assert.Equal(t, "order.placed", e.Type())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for order event")
	}
	select {
	case e := <-userReceived:
		assert.Equal(t, "user.created", e.Type())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for user event")
	}

	assert.Len(t, session.markedMsgs, 2)
}
