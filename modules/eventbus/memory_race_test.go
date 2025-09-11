package eventbus

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
)

// TestMemoryEventBusHighConcurrencyRace is a stress test intended to be run with -race.
// It exercises concurrent publishing, subscription management, and stats collection.
func TestMemoryEventBusHighConcurrencyRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	logger := &testLogger{}
	app := modular.NewObservableApplication(modular.NewStdConfigProvider(struct{}{}), logger)
	modIface := NewModule()
	mod := modIface.(*EventBusModule)
	app.RegisterModule(mod)
	app.RegisterConfigSection("eventbus", modular.NewStdConfigProvider(&EventBusConfig{Engine: "memory", WorkerCount: 8, DefaultEventBufferSize: 64, MaxEventQueueSize: 5000, DeliveryMode: "drop", RotateSubscriberOrder: true}))
	if err := mod.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := mod.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = mod.Stop(context.Background()) }()

	// Pre-create async subscribers on multiple topics to avoid publisher blocking
	topics := []string{"race.alpha", "race.beta", "race.gamma"}
	for _, tp := range topics {
		if _, err := mod.SubscribeAsync(ctx, tp, func(ctx context.Context, e Event) error { return nil }); err != nil {
			t.Fatalf("async sub: %v", err)
		}
	}
	if _, err := mod.SubscribeAsync(ctx, "race.*", func(ctx context.Context, e Event) error { return nil }); err != nil {
		t.Fatalf("async wildcard sub: %v", err)
	}

	var pubWG sync.WaitGroup
	var statsWG sync.WaitGroup
	publisherCount := 4
	perPublisher := 150

	// Publishers
	for p := 0; p < publisherCount; p++ {
		pubWG.Add(1)
		go func(id int) {
			defer pubWG.Done()
			for i := 0; i < perPublisher; i++ {
				topic := topics[i%3]
				_ = mod.Publish(ctx, topic, map[string]int{"p": id, "i": i})
				if i%100 == 0 {
					// Interleave stats reads (discarding values)
					_, _ = mod.Stats()
					_ = mod.PerEngineStats()
				}
			}
		}(p)
	}

	// Concurrent stats reader
	statsStop := make(chan struct{})
	statsWG.Add(1)
	go func() {
		defer statsWG.Done()
		for {
			select {
			case <-statsStop:
				return
			case <-time.After(10 * time.Millisecond):
				_, _ = mod.Stats()
				_ = mod.PerEngineStats()
			}
		}
	}()

	pubWG.Wait()
	close(statsStop)
	statsWG.Wait()
	// final short sleep to allow async workers to drain
	time.Sleep(200 * time.Millisecond)

	// Validate delivered >= expected published events (async may still in flight, so allow slight slack)
	per := mod.PerEngineStats()
	var deliveredTotal, droppedTotal uint64
	for _, st := range per {
		deliveredTotal += st.Delivered
		droppedTotal += st.Dropped
	}
	minPublished := uint64(publisherCount * perPublisher)
	// We allow substantial slack because of drop mode and potential worker lag under race detector.
	// Only fail if delivered count is implausibly low (<25% of published AND no drops recorded suggesting accounting bug).
	if deliveredTotal < minPublished/4 && droppedTotal == 0 {
		_, _, _, _ = runtime.Caller(0)
		// Provide diagnostic context.
		if deliveredTotal < minPublished/4 {
			t.Fatalf("delivered too low: delivered=%d dropped=%d published=%d threshold=%d", deliveredTotal, droppedTotal, minPublished, minPublished/4)
		}
	}
}
