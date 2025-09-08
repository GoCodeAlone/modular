package modular

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDebugRaceCondition helps debug what's happening with the race condition
func TestDebugRaceCondition(t *testing.T) {
	t.Run("debug_concurrent_reloads", func(t *testing.T) {
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		// Create a module that takes some time to reload
		slowModule := &testSlowReloadModule{
			name:        "slow-module",
			reloadDelay: 50 * time.Millisecond, // Reduced delay for faster testing
			reloadCount: 0,
		}

		err := orchestrator.RegisterModule("slow", slowModule)
		require.NoError(t, err)

		concurrency := 10 // Reduced for easier debugging
		var wg sync.WaitGroup
		var successCount int64
		var alreadyProcessingCount int64
		var queueFullCount int64
		var timeoutCount int64
		var otherErrorCount int64

		t.Logf("Starting %d concurrent requests", concurrency)

		// Launch concurrent reload requests
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				start := time.Now()
				err := orchestrator.RequestReload(ctx)
				duration := time.Since(start)

				t.Logf("Request %d completed in %v with error: %v", id, duration, err)

				if err != nil {
					if err.Error() == "reload orchestrator: reload already in progress" {
						atomic.AddInt64(&alreadyProcessingCount, 1)
					} else if err.Error() == "reload orchestrator: request queue is full" {
						atomic.AddInt64(&queueFullCount, 1)
					} else if err == context.DeadlineExceeded {
						atomic.AddInt64(&timeoutCount, 1)
					} else {
						atomic.AddInt64(&otherErrorCount, 1)
						t.Logf("Request %d had unexpected error: %v", id, err)
					}
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		finalSuccessCount := atomic.LoadInt64(&successCount)
		finalAlreadyProcessingCount := atomic.LoadInt64(&alreadyProcessingCount)
		finalQueueFullCount := atomic.LoadInt64(&queueFullCount)
		finalTimeoutCount := atomic.LoadInt64(&timeoutCount)
		finalOtherErrorCount := atomic.LoadInt64(&otherErrorCount)
		finalReloadCount := slowModule.getReloadCount()

		t.Logf("Results:")
		t.Logf("  Success count: %d", finalSuccessCount)
		t.Logf("  Already processing count: %d", finalAlreadyProcessingCount)
		t.Logf("  Queue full count: %d", finalQueueFullCount)
		t.Logf("  Timeout count: %d", finalTimeoutCount)
		t.Logf("  Other error count: %d", finalOtherErrorCount)
		t.Logf("  Module reload count: %d", finalReloadCount)
		t.Logf("  Total accounted: %d", finalSuccessCount+finalAlreadyProcessingCount+finalQueueFullCount+finalTimeoutCount+finalOtherErrorCount)

		// With proper concurrency control, we should see:
		// - 1 success
		// - (concurrency-1) already processing errors
		// - 1 module reload
		if finalSuccessCount > 1 {
			t.Errorf("Expected at most 1 success, got %d - race condition may still exist", finalSuccessCount)
		}
		if finalReloadCount != finalSuccessCount {
			t.Errorf("Module reload count (%d) should match success count (%d)", finalReloadCount, finalSuccessCount)
		}
	})
}
