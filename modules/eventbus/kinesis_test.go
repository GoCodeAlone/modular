package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/GoCodeAlone/modular/modules/eventbus/mocks"
)

// newTestKinesisEventBus creates a KinesisEventBus wired to a mock client,
// pre-started so Publish() can be called immediately.
func newTestKinesisEventBus(client KinesisClient) *KinesisEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: DefaultKinesisPollInterval,
		},
		client:        client,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}
}

func TestKinesisPublishPartitionKey(t *testing.T) {
	t.Run("uses context partition key when set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			PutRecord(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
				assert.Equal(t, "user-42", *input.PartitionKey)
				assert.Equal(t, "test-stream", *input.StreamName)
				return &kinesis.PutRecordOutput{}, nil
			})

		ctx := WithPartitionKey(context.Background(), "user-42")
		err := bus.Publish(ctx, newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("falls back to topic when no context key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			PutRecord(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
				assert.Equal(t, "orders.created", *input.PartitionKey)
				return &kinesis.PutRecordOutput{}, nil
			})

		err := bus.Publish(context.Background(), newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("falls back to topic when context key is empty string", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			PutRecord(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
				assert.Equal(t, "orders.created", *input.PartitionKey,
					"empty string partition key should fall back to topic for Kinesis")
				return &kinesis.PutRecordOutput{}, nil
			})

		ctx := WithPartitionKey(context.Background(), "")
		err := bus.Publish(ctx, newTestCloudEvent("orders.created", "data"))
		require.NoError(t, err)
	})

	t.Run("propagates PutRecord error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		m.EXPECT().
			PutRecord(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("throttled"))

		err := bus.Publish(context.Background(), newTestCloudEvent("test", "data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "throttled")
	})
}

func TestKinesisPublishCloudEventsWireFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockKinesisClient(ctrl)
	bus := newTestKinesisEventBus(m)
	defer bus.cancel()

	m.EXPECT().
		PutRecord(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
			var wire map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(input.Data, &wire))

			assert.Contains(t, wire, "specversion", "wire format should have specversion at top level")
			assert.Contains(t, wire, "type", "wire format should have type at top level")
			assert.Contains(t, wire, "source", "wire format should have source at top level")
			assert.NotContains(t, wire, "topic", "wire format should NOT have Event.Topic wrapper")
			assert.NotContains(t, wire, "payload", "wire format should NOT have Event.Payload wrapper")
			assert.NotContains(t, wire, "metadata", "wire format should NOT have Event.Metadata wrapper")
			return &kinesis.PutRecordOutput{}, nil
		})

	e := newTestCloudEvent("messaging.texter-message.received", map[string]interface{}{"messageId": "msg-456"})
	e.SetSource("/chimera/messaging")
	e.SetID("evt-123")

	err := bus.Publish(context.Background(), e)
	require.NoError(t, err)
}

func TestKinesisStart(t *testing.T) {
	t.Run("succeeds when stream already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := &KinesisEventBus{
			config:        &KinesisConfig{StreamName: "my-stream", ShardCount: 2},
			client:        m,
			subscriptions: make(map[string]map[string]*kinesisSubscription),
		}

		m.EXPECT().
			DescribeStream(gomock.Any(), gomock.Any()).
			Return(&kinesis.DescribeStreamOutput{}, nil)

		err := bus.Start(context.Background())
		require.NoError(t, err)
		assert.True(t, bus.isStarted)
	})

	t.Run("returns nil when already started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		// No EXPECT calls — nothing should be called
		err := bus.Start(context.Background())
		require.NoError(t, err)
	})

	t.Run("returns error for invalid shard count when stream missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := &KinesisEventBus{
			config:        &KinesisConfig{StreamName: "my-stream", ShardCount: 0},
			client:        m,
			subscriptions: make(map[string]map[string]*kinesisSubscription),
		}

		m.EXPECT().
			DescribeStream(gomock.Any(), gomock.Any()).
			Return(&kinesis.DescribeStreamOutput{}, fmt.Errorf("stream not found"))

		err := bus.Start(context.Background())
		assert.ErrorIs(t, err, ErrInvalidShardCount)
	})

	t.Run("propagates CreateStream error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := &KinesisEventBus{
			config:        &KinesisConfig{StreamName: "my-stream", ShardCount: 2},
			client:        m,
			subscriptions: make(map[string]map[string]*kinesisSubscription),
		}

		m.EXPECT().
			DescribeStream(gomock.Any(), gomock.Any()).
			Return(&kinesis.DescribeStreamOutput{}, fmt.Errorf("stream not found"))
		m.EXPECT().
			CreateStream(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("access denied"))

		err := bus.Start(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
		assert.False(t, bus.isStarted)
	})
}

func TestKinesisStop(t *testing.T) {
	t.Run("returns nil when not started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := &KinesisEventBus{
			config:        &KinesisConfig{StreamName: "test-stream"},
			client:        m,
			subscriptions: make(map[string]map[string]*kinesisSubscription),
		}

		err := bus.Stop(context.Background())
		require.NoError(t, err)
	})

	t.Run("clears subscriptions and marks stopped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)

		err := bus.Stop(context.Background())
		require.NoError(t, err)
		assert.False(t, bus.isStarted)
		assert.Empty(t, bus.subscriptions)
	})

	t.Run("returns timeout error when context expires", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)

		// Add a wait group entry that never completes to simulate a stuck worker
		bus.wg.Add(1)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err := bus.Stop(ctx)
		assert.ErrorIs(t, err, ErrEventBusShutdownTimeout)

		// Clean up the stuck worker
		bus.wg.Done()
	})
}

func TestKinesisSubscribe(t *testing.T) {
	t.Run("returns error when not started", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := &KinesisEventBus{
			config:        &KinesisConfig{StreamName: "test-stream"},
			client:        m,
			subscriptions: make(map[string]map[string]*kinesisSubscription),
		}

		_, err := bus.Subscribe(context.Background(), "topic", func(ctx context.Context, event Event) error { return nil })
		assert.ErrorIs(t, err, ErrEventBusNotStarted)
	})

	t.Run("returns error for nil handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mocks.NewMockKinesisClient(ctrl)
		bus := newTestKinesisEventBus(m)
		defer bus.cancel()

		_, err := bus.Subscribe(context.Background(), "topic", nil)
		assert.ErrorIs(t, err, ErrEventHandlerNil)
	})
}

func TestKinesisPublishNotStarted(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockKinesisClient(ctrl)
	bus := &KinesisEventBus{
		config:        &KinesisConfig{StreamName: "test-stream"},
		client:        m,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
	}

	err := bus.Publish(context.Background(), newTestCloudEvent("test", "data"))
	assert.ErrorIs(t, err, ErrEventBusNotStarted)
}

// --- isExpiredIteratorError unit tests ---

func TestIsExpiredIteratorError(t *testing.T) {
	t.Run("returns true for ExpiredIteratorException", func(t *testing.T) {
		msg := "Iterator expired"
		err := &types.ExpiredIteratorException{Message: &msg}
		assert.True(t, isExpiredIteratorError(err))
	})

	t.Run("returns true for wrapped ExpiredIteratorException", func(t *testing.T) {
		msg := "Iterator expired"
		inner := &types.ExpiredIteratorException{Message: &msg}
		wrapped := fmt.Errorf("kinesis error: %w", inner)
		assert.True(t, isExpiredIteratorError(wrapped))
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		assert.False(t, isExpiredIteratorError(errors.New("something else")))
	})

	t.Run("returns false for other Kinesis errors", func(t *testing.T) {
		msg := "Throughput exceeded"
		err := &types.ProvisionedThroughputExceededException{Message: &msg}
		assert.False(t, isExpiredIteratorError(err))
	})
}

// --- ReadShard integration tests: CloudEvents deserialization via mock Kinesis ---

// newReadShardTestBus creates a test bus with subscriptions wired directly
// (bypasses Subscribe to avoid starting shard reader goroutines).
func newReadShardTestBus(t *testing.T, client KinesisClient, subs map[string]map[string]*kinesisSubscription) *KinesisEventBus {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: DefaultKinesisPollInterval,
		},
		client:        client,
		subscriptions: subs,
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}
}

func TestKinesisReadShardCloudEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	received := make(chan Event, 1)
	subs := map[string]map[string]*kinesisSubscription{
		"order.placed": {
			"test-sub": {
				id:    "test-sub",
				topic: "order.placed",
				handler: func(ctx context.Context, event Event) error {
					received <- event
					return nil
				},
				done: make(chan struct{}),
			},
		},
	}
	bus := newReadShardTestBus(t, mockClient, subs)
	subs["order.placed"]["test-sub"].bus = bus

	ceJSON := []byte(`{
		"specversion": "1.0",
		"type": "order.placed",
		"source": "order-service",
		"id": "evt-001",
		"time": "2026-02-06T12:00:00Z",
		"data": {"orderId": "abc-123"}
	}`)

	iteratorStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iteratorStr}, nil)

	// Return records then nil iterator to terminate the loop.
	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetRecordsOutput{
			Records:           []types.Record{{Data: ceJSON}},
			NextShardIterator: nil,
		}, nil)

	bus.wg.Add(1)
	bus.readShard("shard-0")

	select {
	case event := <-received:
		assert.Equal(t, "order.placed", event.Type())
		assert.Equal(t, "1.0", event.SpecVersion())
		assert.Equal(t, "order-service", event.Source())
		assert.Equal(t, "evt-001", event.ID())
		var payloadMap map[string]interface{}
		require.NoError(t, event.DataAs(&payloadMap))
		assert.Equal(t, "abc-123", payloadMap["orderId"])
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestKinesisReadShardCloudEventBase64(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	received := make(chan Event, 1)
	subs := map[string]map[string]*kinesisSubscription{
		"file.uploaded": {
			"test-sub": {
				id:    "test-sub",
				topic: "file.uploaded",
				handler: func(ctx context.Context, event Event) error {
					received <- event
					return nil
				},
				done: make(chan struct{}),
			},
		},
	}
	bus := newReadShardTestBus(t, mockClient, subs)
	subs["file.uploaded"]["test-sub"].bus = bus

	// "SGVsbG8gV29ybGQ=" is base64 for "Hello World"
	ceJSON := []byte(`{"specversion":"1.0","type":"file.uploaded","source":"storage-service","id":"evt-002","data_base64":"SGVsbG8gV29ybGQ="}`)

	iteratorStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iteratorStr}, nil)

	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetRecordsOutput{
			Records:           []types.Record{{Data: ceJSON}},
			NextShardIterator: nil,
		}, nil)

	bus.wg.Add(1)
	bus.readShard("shard-0")

	select {
	case event := <-received:
		assert.Equal(t, "file.uploaded", event.Type())
		assert.Equal(t, []byte("Hello World"), event.Data())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestKinesisReadShardRejectsInvalidSpecversion(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	handlerCalled := make(chan struct{}, 1)
	subs := map[string]map[string]*kinesisSubscription{
		"order.placed": {
			"test-sub": {
				id:    "test-sub",
				topic: "order.placed",
				handler: func(ctx context.Context, event Event) error {
					handlerCalled <- struct{}{}
					return nil
				},
				done: make(chan struct{}),
			},
		},
	}
	bus := newReadShardTestBus(t, mockClient, subs)
	subs["order.placed"]["test-sub"].bus = bus

	badCE := []byte(`{
		"specversion": "99.9",
		"type": "order.placed",
		"source": "order-service",
		"id": "evt-bad"
	}`)

	iteratorStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iteratorStr}, nil)

	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetRecordsOutput{
			Records:           []types.Record{{Data: badCE}},
			NextShardIterator: nil,
		}, nil)

	bus.wg.Add(1)
	bus.readShard("shard-0")

	// Handler should NOT have been called for invalid specversion.
	select {
	case <-handlerCalled:
		t.Fatal("handler should not have been called for invalid specversion")
	case <-time.After(100 * time.Millisecond):
		// Success: handler was not called.
	}
}

func TestKinesisReadShardMultipleRecords(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	orderReceived := make(chan Event, 1)
	userReceived := make(chan Event, 1)
	subs := map[string]map[string]*kinesisSubscription{
		"order.placed": {
			"order-sub": {
				id:    "order-sub",
				topic: "order.placed",
				handler: func(ctx context.Context, e Event) error {
					orderReceived <- e
					return nil
				},
				done: make(chan struct{}),
			},
		},
		"user.created": {
			"user-sub": {
				id:    "user-sub",
				topic: "user.created",
				handler: func(ctx context.Context, e Event) error {
					userReceived <- e
					return nil
				},
				done: make(chan struct{}),
			},
		},
	}
	bus := newReadShardTestBus(t, mockClient, subs)
	subs["order.placed"]["order-sub"].bus = bus
	subs["user.created"]["user-sub"].bus = bus

	iteratorStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iteratorStr}, nil)

	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetRecordsOutput{
			Records: []types.Record{
				{Data: []byte(`{"specversion":"1.0","type":"order.placed","source":"orders","id":"1","data":{"orderId":"o1"}}`)},
				{Data: []byte(`{"specversion":"1.0","type":"user.created","source":"users","id":"2","data":{"userId":"u1"}}`)},
			},
			NextShardIterator: nil,
		}, nil)

	bus.wg.Add(1)
	bus.readShard("shard-0")

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
}

// --- Expired iterator recovery integration tests ---

func TestKinesisReadShardExpiredIteratorRecovery(t *testing.T) {
	t.Run("recovers with LATEST when no sequence number tracked", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := mocks.NewMockKinesisClient(ctrl)

		received := make(chan Event, 1)
		subs := map[string]map[string]*kinesisSubscription{
			"order.placed": {
				"test-sub": {
					id:    "test-sub",
					topic: "order.placed",
					handler: func(ctx context.Context, event Event) error {
						received <- event
						return nil
					},
					done: make(chan struct{}),
				},
			},
		}
		bus := newReadShardTestBus(t, mockClient, subs)
		subs["order.placed"]["test-sub"].bus = bus

		initialIter := "shard-iter-1"
		refreshedIter := "shard-iter-2"

		// 1. Initial GetShardIterator succeeds
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.GetShardIteratorInput, optFns ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error) {
				assert.Equal(t, types.ShardIteratorTypeLatest, input.ShardIteratorType)
				return &kinesis.GetShardIteratorOutput{ShardIterator: &initialIter}, nil
			})

		// 2. First GetRecords returns ExpiredIteratorException
		expiredMsg := "Iterator expired because it aged past 5 minutes"
		gomock.InOrder(
			mockClient.EXPECT().
				GetRecords(gomock.Any(), gomock.Any()).
				Return(nil, &types.ExpiredIteratorException{Message: &expiredMsg}),
		)

		// 3. Refresh GetShardIterator — should use LATEST since no records were processed
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.GetShardIteratorInput, optFns ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error) {
				assert.Equal(t, types.ShardIteratorTypeLatest, input.ShardIteratorType,
					"should use LATEST when no sequence number has been tracked")
				assert.Nil(t, input.StartingSequenceNumber)
				return &kinesis.GetShardIteratorOutput{ShardIterator: &refreshedIter}, nil
			})

		// 4. Second GetRecords succeeds with data, then nil iterator to terminate
		ceJSON := []byte(`{"specversion":"1.0","type":"order.placed","source":"orders","id":"evt-recover","data":{"orderId":"recovered"}}`)
		mockClient.EXPECT().
			GetRecords(gomock.Any(), gomock.Any()).
			Return(&kinesis.GetRecordsOutput{
				Records:           []types.Record{{Data: ceJSON}},
				NextShardIterator: nil,
			}, nil)

		bus.wg.Add(1)
		bus.readShard("shard-0")

		select {
		case event := <-received:
			assert.Equal(t, "order.placed", event.Type())
			var payloadMap map[string]interface{}
			require.NoError(t, event.DataAs(&payloadMap))
			assert.Equal(t, "recovered", payloadMap["orderId"])
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event after iterator recovery")
		}
	})

	t.Run("recovers with AFTER_SEQUENCE_NUMBER when records were previously processed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := mocks.NewMockKinesisClient(ctrl)

		received := make(chan Event, 2)
		subs := map[string]map[string]*kinesisSubscription{
			"order.placed": {
				"test-sub": {
					id:    "test-sub",
					topic: "order.placed",
					handler: func(ctx context.Context, event Event) error {
						received <- event
						return nil
					},
					done: make(chan struct{}),
				},
			},
		}
		bus := newReadShardTestBus(t, mockClient, subs)
		subs["order.placed"]["test-sub"].bus = bus

		initialIter := "shard-iter-1"
		secondIter := "shard-iter-2"
		refreshedIter := "shard-iter-3"
		seqNum := "49607379238952109838144426"

		// 1. Initial GetShardIterator
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			Return(&kinesis.GetShardIteratorOutput{ShardIterator: &initialIter}, nil)

		// 2. First GetRecords succeeds with a record (establishes lastSeqNum)
		firstRecord := []byte(`{"specversion":"1.0","type":"order.placed","source":"orders","id":"evt-1","data":{"orderId":"first"}}`)
		mockClient.EXPECT().
			GetRecords(gomock.Any(), gomock.Any()).
			Return(&kinesis.GetRecordsOutput{
				Records:           []types.Record{{Data: firstRecord, SequenceNumber: &seqNum}},
				NextShardIterator: &secondIter,
			}, nil)

		// 3. Second GetRecords returns ExpiredIteratorException
		expiredMsg := "Iterator expired"
		mockClient.EXPECT().
			GetRecords(gomock.Any(), gomock.Any()).
			Return(nil, &types.ExpiredIteratorException{Message: &expiredMsg})

		// 4. Refresh GetShardIterator — should use AFTER_SEQUENCE_NUMBER with the tracked seq num
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, input *kinesis.GetShardIteratorInput, optFns ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error) {
				assert.Equal(t, types.ShardIteratorTypeAfterSequenceNumber, input.ShardIteratorType,
					"should use AFTER_SEQUENCE_NUMBER when a sequence number was tracked")
				require.NotNil(t, input.StartingSequenceNumber)
				assert.Equal(t, seqNum, *input.StartingSequenceNumber)
				return &kinesis.GetShardIteratorOutput{ShardIterator: &refreshedIter}, nil
			})

		// 5. Third GetRecords succeeds, then nil iterator to terminate
		secondRecord := []byte(`{"specversion":"1.0","type":"order.placed","source":"orders","id":"evt-2","data":{"orderId":"second"}}`)
		mockClient.EXPECT().
			GetRecords(gomock.Any(), gomock.Any()).
			Return(&kinesis.GetRecordsOutput{
				Records:           []types.Record{{Data: secondRecord}},
				NextShardIterator: nil,
			}, nil)

		bus.wg.Add(1)
		bus.readShard("shard-0")

		// Should receive both events
		for i, expectedID := range []string{"first", "second"} {
			select {
			case event := <-received:
				assert.Equal(t, "order.placed", event.Type())
				var payloadMap map[string]interface{}
				require.NoError(t, event.DataAs(&payloadMap), "event %d payload should be a map", i)
				assert.Equal(t, expectedID, payloadMap["orderId"])
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for event %d", i)
			}
		}
	})

	t.Run("exits cleanly on context cancellation during refresh backoff", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := mocks.NewMockKinesisClient(ctrl)

		subs := map[string]map[string]*kinesisSubscription{
			"order.placed": {
				"test-sub": {
					id:    "test-sub",
					topic: "order.placed",
					handler: func(ctx context.Context, event Event) error {
						return nil
					},
					done: make(chan struct{}),
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		bus := &KinesisEventBus{
			config: &KinesisConfig{
				StreamName:   "test-stream",
				ShardCount:   1,
				PollInterval: DefaultKinesisPollInterval,
			},
			client:        mockClient,
			subscriptions: subs,
			activeShards:  make(map[string]struct{}),
			ctx:           ctx,
			cancel:        cancel,
			isStarted:     true,
		}
		subs["order.placed"]["test-sub"].bus = bus

		initialIter := "shard-iter-1"

		// 1. Initial GetShardIterator
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			Return(&kinesis.GetShardIteratorOutput{ShardIterator: &initialIter}, nil)

		// 2. GetRecords returns ExpiredIteratorException
		expiredMsg := "Iterator expired"
		mockClient.EXPECT().
			GetRecords(gomock.Any(), gomock.Any()).
			Return(nil, &types.ExpiredIteratorException{Message: &expiredMsg})

		// 3. Refresh fails — triggers the 5s backoff timer
		mockClient.EXPECT().
			GetShardIterator(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("service unavailable"))

		// Cancel context shortly after to test that readShard exits during backoff
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		done := make(chan struct{})
		bus.wg.Add(1)
		go func() {
			bus.readShard("shard-0")
			close(done)
		}()

		select {
		case <-done:
			// readShard exited cleanly during backoff — success
		case <-time.After(3 * time.Second):
			t.Fatal("readShard did not exit after context cancellation during refresh backoff")
		}
	})
}

// --- Shard tracking and deduplication tests ---

func TestKinesisStartShardReadersSkipsActiveShards(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: 10 * time.Millisecond,
		},
		client:        mockClient,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}

	shardID := "shardId-000000000000"

	// Pre-mark the shard as active (simulating an already-running reader)
	bus.shardMutex.Lock()
	bus.activeShards[shardID] = struct{}{}
	bus.shardMutex.Unlock()

	describeCalled := make(chan struct{}, 1)

	// DescribeStream returns the same shard that's already active
	mockClient.EXPECT().
		DescribeStream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
			describeCalled <- struct{}{}
			return &kinesis.DescribeStreamOutput{
				StreamDescription: &types.StreamDescription{
					Shards: []types.Shard{{ShardId: &shardID}},
				},
			}, nil
		}).
		AnyTimes()

	// GetShardIterator should NEVER be called — the shard is already active
	// (gomock will fail the test if this is unexpectedly called)

	bus.startShardReaders()

	// Wait for scanner to complete its first DescribeStream call
	select {
	case <-describeCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not call DescribeStream")
	}

	// Give a moment for any (incorrect) goroutine spawn to happen
	time.Sleep(50 * time.Millisecond)

	cancel()
	bus.wg.Wait()

	// If startShardReaders had spawned a duplicate reader, gomock would fail
	// due to an unexpected GetShardIterator call.
}

func TestKinesisStartShardReadersDescribeStreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: DefaultKinesisPollInterval,
		},
		client:        mockClient,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}

	describeCalled := make(chan struct{}, 1)

	// DescribeStream returns an error — scanner should retry after backoff
	mockClient.EXPECT().
		DescribeStream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
			describeCalled <- struct{}{}
			return nil, fmt.Errorf("access denied")
		}).
		AnyTimes()

	bus.startShardReaders()

	// Wait for the first DescribeStream call
	select {
	case <-describeCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not call DescribeStream")
	}

	// Cancel during the 5s error backoff — should exit promptly
	cancel()

	done := make(chan struct{})
	go func() {
		bus.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not exit promptly after context cancellation during error backoff")
	}
}

func TestKinesisStartShardReadersNilStreamDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: DefaultKinesisPollInterval,
		},
		client:        mockClient,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}

	describeCalled := make(chan struct{}, 1)

	// DescribeStream succeeds but returns nil StreamDescription
	mockClient.EXPECT().
		DescribeStream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
			describeCalled <- struct{}{}
			return &kinesis.DescribeStreamOutput{StreamDescription: nil}, nil
		}).
		AnyTimes()

	bus.startShardReaders()

	// Wait for the first DescribeStream call
	select {
	case <-describeCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not call DescribeStream")
	}

	// Cancel during the 5s nil-description backoff — should exit promptly
	cancel()

	done := make(chan struct{})
	go func() {
		bus.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not exit promptly after context cancellation during nil description backoff")
	}
}

func TestKinesisStartShardReadersExitsOnContextCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	bus := &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: DefaultKinesisPollInterval,
		},
		client:        mockClient,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}

	// Cancel immediately before scanner can call DescribeStream
	cancel()

	bus.startShardReaders()

	done := make(chan struct{})
	go func() {
		bus.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not exit on pre-cancelled context")
	}
}

func TestKinesisReadShardCleansUpActiveShards(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	bus := newTestKinesisEventBus(mockClient)
	defer bus.cancel()

	shardID := "shard-0"

	// Pre-register the shard as active (simulating what startShardReaders does)
	bus.shardMutex.Lock()
	bus.activeShards[shardID] = struct{}{}
	bus.shardMutex.Unlock()

	iterStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iterStr}, nil)

	// Return nil NextShardIterator to make readShard exit immediately
	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetRecordsOutput{
			Records:           []types.Record{},
			NextShardIterator: nil,
		}, nil)

	bus.wg.Add(1)
	bus.readShard(shardID)

	// After readShard exits, the shard should be removed from activeShards
	bus.shardMutex.Lock()
	_, stillActive := bus.activeShards[shardID]
	bus.shardMutex.Unlock()

	assert.False(t, stillActive, "shard should be removed from activeShards after readShard exits")
}

func TestKinesisShardScanOncePreventsDuplicateScanners(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	bus := newTestKinesisEventBus(mockClient)
	defer bus.cancel()

	describeCalled := make(chan struct{}, 3)
	var describeCallCount int32

	// DescribeStream should only be called by ONE scanner goroutine.
	mockClient.EXPECT().
		DescribeStream(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
			atomic.AddInt32(&describeCallCount, 1)
			describeCalled <- struct{}{}
			// Return no shards to keep things simple
			return &kinesis.DescribeStreamOutput{
				StreamDescription: &types.StreamDescription{
					Shards: []types.Shard{},
				},
			}, nil
		}).
		AnyTimes()

	// Subscribe three times — each triggers shardScanOnce.Do
	handler := func(ctx context.Context, event Event) error { return nil }
	_, err := bus.Subscribe(context.Background(), "topic.a", handler)
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "topic.b", handler)
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "topic.c", handler)
	require.NoError(t, err)

	// Wait for the first DescribeStream call (channel-based, not time-based)
	select {
	case <-describeCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("scanner did not call DescribeStream")
	}

	// Give a brief window for any duplicate scanners to call DescribeStream
	time.Sleep(50 * time.Millisecond)
	bus.cancel()
	bus.wg.Wait()

	// With sync.Once, only one scanner goroutine should have been launched.
	count := atomic.LoadInt32(&describeCallCount)
	assert.Equal(t, int32(1), count,
		"only one shard scanner should run regardless of subscribe count")
}

func TestKinesisStartInitializesActiveShards(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mocks.NewMockKinesisClient(ctrl)
	bus := &KinesisEventBus{
		config:        &KinesisConfig{StreamName: "my-stream", ShardCount: 1},
		client:        m,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
	}

	m.EXPECT().
		DescribeStream(gomock.Any(), gomock.Any()).
		Return(&kinesis.DescribeStreamOutput{}, nil)

	err := bus.Start(context.Background())
	require.NoError(t, err)
	defer bus.cancel()

	assert.NotNil(t, bus.activeShards, "Start should initialize activeShards map")
	assert.Equal(t, DefaultKinesisPollInterval, bus.config.PollInterval,
		"Start should set default PollInterval when not configured")
}

func TestKinesisNewEventBusPollInterval(t *testing.T) {
	t.Run("parses valid poll interval", func(t *testing.T) {
		config := map[string]interface{}{
			"pollInterval": "500ms",
		}
		bus, err := NewKinesisEventBus(config)
		require.NoError(t, err)

		kBus := bus.(*KinesisEventBus)
		assert.Equal(t, 500*time.Millisecond, kBus.config.PollInterval)
	})

	t.Run("defaults to 1s when not set", func(t *testing.T) {
		config := map[string]interface{}{}
		bus, err := NewKinesisEventBus(config)
		require.NoError(t, err)

		kBus := bus.(*KinesisEventBus)
		assert.Equal(t, DefaultKinesisPollInterval, kBus.config.PollInterval)
	})

	t.Run("returns error for invalid poll interval", func(t *testing.T) {
		config := map[string]interface{}{
			"pollInterval": "not-a-duration",
		}
		_, err := NewKinesisEventBus(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pollInterval")
	})

	t.Run("returns error for negative poll interval", func(t *testing.T) {
		config := map[string]interface{}{
			"pollInterval": "-500ms",
		}
		_, err := NewKinesisEventBus(config)
		assert.ErrorIs(t, err, ErrInvalidPollInterval)
	})

	t.Run("returns error for zero poll interval", func(t *testing.T) {
		config := map[string]interface{}{
			"pollInterval": "0s",
		}
		_, err := NewKinesisEventBus(config)
		assert.ErrorIs(t, err, ErrInvalidPollInterval)
	})
}

func TestKinesisReadShardUsesConfiguredPollInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mocks.NewMockKinesisClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := &KinesisEventBus{
		config: &KinesisConfig{
			StreamName:   "test-stream",
			ShardCount:   1,
			PollInterval: 50 * time.Millisecond,
		},
		client:        mockClient,
		subscriptions: make(map[string]map[string]*kinesisSubscription),
		activeShards:  make(map[string]struct{}),
		ctx:           ctx,
		cancel:        cancel,
		isStarted:     true,
	}

	iterStr := "shard-iter-1"
	mockClient.EXPECT().
		GetShardIterator(gomock.Any(), gomock.Any()).
		Return(&kinesis.GetShardIteratorOutput{ShardIterator: &iterStr}, nil)

	timestamps := make(chan time.Time, 5)
	callCount := 0
	mockClient.EXPECT().
		GetRecords(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, input *kinesis.GetRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error) {
			timestamps <- time.Now()
			callCount++
			if callCount >= 4 {
				cancel()
				return &kinesis.GetRecordsOutput{
					Records:           []types.Record{},
					NextShardIterator: nil,
				}, nil
			}
			nextIter := fmt.Sprintf("shard-iter-%d", callCount+1)
			return &kinesis.GetRecordsOutput{
				Records:           []types.Record{},
				NextShardIterator: &nextIter,
			}, nil
		}).
		AnyTimes()

	bus.wg.Add(1)
	bus.readShard("shard-0")
	close(timestamps)

	// Collect timestamps and verify intervals are approximately 50ms
	var times []time.Time
	for ts := range timestamps {
		times = append(times, ts)
	}

	require.GreaterOrEqual(t, len(times), 3, "should have at least 3 poll cycles")

	for i := 1; i < len(times)-1; i++ {
		interval := times[i].Sub(times[i-1])
		assert.GreaterOrEqual(t, interval, 40*time.Millisecond,
			"poll interval %d should be at least ~50ms (got %v)", i, interval)
		assert.LessOrEqual(t, interval, 200*time.Millisecond,
			"poll interval %d should not be excessively long (got %v)", i, interval)
	}
}
