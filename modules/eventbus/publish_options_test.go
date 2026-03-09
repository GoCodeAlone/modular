package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithPartitionKey(t *testing.T) {
	t.Run("sets partition key in context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithPartitionKey(ctx, "user-123")

		key, ok := PartitionKeyFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "user-123", key)
	})

	t.Run("returns false when not set", func(t *testing.T) {
		ctx := context.Background()

		key, ok := PartitionKeyFromContext(ctx)
		assert.False(t, ok)
		assert.Equal(t, "", key)
	})

	t.Run("empty string is preserved in context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithPartitionKey(ctx, "")

		key, ok := PartitionKeyFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "", key)
	})

	t.Run("later call overrides earlier", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithPartitionKey(ctx, "first")
		ctx = WithPartitionKey(ctx, "second")

		key, ok := PartitionKeyFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, "second", key)
	})
}

func TestPublishWithPartitionKey(t *testing.T) {
	t.Run("publish with partition key succeeds", func(t *testing.T) {
		module := NewModule().(*EventBusModule)
		app := newMockApp()

		cfg := &EventBusConfig{
			Engine:                 "memory",
			MaxEventQueueSize:      100,
			DefaultEventBufferSize: 10,
			WorkerCount:            2,
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))

		err := module.Init(app)
		require.NoError(t, err)

		ctx := context.Background()
		err = module.Start(ctx)
		require.NoError(t, err)
		defer func() {
			_ = module.Stop(ctx)
		}()

		eventReceived := make(chan Event, 1)
		_, err = module.Subscribe(ctx, "test.partitioned", func(ctx context.Context, event Event) error {
			eventReceived <- event
			return nil
		})
		require.NoError(t, err)

		// Use context to set partition key
		pubCtx := WithPartitionKey(ctx, "custom-key")
		err = module.Publish(pubCtx, "test.partitioned", "test-payload")
		require.NoError(t, err)

		select {
		case event := <-eventReceived:
			var payload string
			require.NoError(t, event.DataAs(&payload))
			assert.Equal(t, "test-payload", payload)
			assert.Equal(t, "test.partitioned", event.Type())
		case <-time.After(2 * time.Second):
			t.Fatal("Event not received within timeout")
		}
	})

	t.Run("publish without partition key still works", func(t *testing.T) {
		module := NewModule().(*EventBusModule)
		app := newMockApp()

		cfg := &EventBusConfig{
			Engine:                 "memory",
			MaxEventQueueSize:      100,
			DefaultEventBufferSize: 10,
			WorkerCount:            2,
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))

		err := module.Init(app)
		require.NoError(t, err)

		ctx := context.Background()
		err = module.Start(ctx)
		require.NoError(t, err)
		defer func() {
			_ = module.Stop(ctx)
		}()

		eventReceived := make(chan Event, 1)
		_, err = module.Subscribe(ctx, "test.basic", func(ctx context.Context, event Event) error {
			eventReceived <- event
			return nil
		})
		require.NoError(t, err)

		// Publish without partition key (backward compatible)
		err = module.Publish(ctx, "test.basic", "basic-payload")
		require.NoError(t, err)

		select {
		case event := <-eventReceived:
			var payload string
			require.NoError(t, event.DataAs(&payload))
			assert.Equal(t, "basic-payload", payload)
		case <-time.After(2 * time.Second):
			t.Fatal("Event not received within timeout")
		}
	})
}

func TestPublishWithPartitionKeyConcurrency(t *testing.T) {
	t.Run("concurrent publishes with different partition keys", func(t *testing.T) {
		module := NewModule().(*EventBusModule)
		app := newMockApp()

		cfg := &EventBusConfig{
			Engine:                 "memory",
			MaxEventQueueSize:      1000,
			DefaultEventBufferSize: 1000,
			WorkerCount:            5,
			DeliveryMode:           "block",
		}
		app.RegisterConfigSection(ModuleName, modular.NewStdConfigProvider(cfg))

		err := module.Init(app)
		require.NoError(t, err)

		ctx := context.Background()
		err = module.Start(ctx)
		require.NoError(t, err)
		defer func() {
			_ = module.Stop(ctx)
		}()

		var receivedCount atomic.Int64
		_, err = module.Subscribe(ctx, "concurrent.topic", func(ctx context.Context, event Event) error {
			receivedCount.Add(1)
			return nil
		})
		require.NoError(t, err)

		const numPublishers = 50
		const messagesPerPublisher = 10
		var wg sync.WaitGroup

		for i := 0; i < numPublishers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < messagesPerPublisher; j++ {
					key := string(rune('a' + (idx % 10)))
					pubCtx := WithPartitionKey(ctx, key)
					pubErr := module.Publish(pubCtx, "concurrent.topic", idx*100+j)
					assert.NoError(t, pubErr)
				}
			}(i)
		}

		wg.Wait()

		// Wait for the handler goroutine to drain all buffered events
		assert.Eventually(t, func() bool {
			return receivedCount.Load() == int64(numPublishers*messagesPerPublisher)
		}, 10*time.Second, 50*time.Millisecond,
			"expected %d events, got %d", numPublishers*messagesPerPublisher, receivedCount.Load())
	})
}
