package modular

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReloadOrchestratorRaceCondition tests for the specific race condition
// identified in handleReloadRequest between checking and setting the processing flag
func TestReloadOrchestratorRaceCondition(t *testing.T) {
	t.Run("should_expose_race_condition_in_processing_flag", func(t *testing.T) {
		// This test is designed to fail with the current implementation
		// to demonstrate the race condition before we fix it

		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		// Create a module that takes some time to reload
		slowModule := &testSlowReloadModule{
			name:        "slow-module",
			reloadDelay: 100 * time.Millisecond,
			reloadCount: 0,
		}

		err := orchestrator.RegisterModule("slow", slowModule)
		require.NoError(t, err)

		// Try to trigger the race condition by launching many concurrent reloads
		// The race condition occurs when multiple goroutines check `processing == false`
		// before any of them can set `processing = true`

		concurrency := runtime.NumCPU() * 4 // High concurrency to increase race chance
		var wg sync.WaitGroup
		var successCount int64
		var alreadyProcessingCount int64

		// Launch concurrent reload requests
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				err := orchestrator.RequestReload(ctx)
				if err != nil {
					if err.Error() == "reload orchestrator: reload already in progress" {
						atomic.AddInt64(&alreadyProcessingCount, 1)
					}
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		// With the race condition, we might see:
		// 1. Multiple successful reloads (when multiple goroutines slip through)
		// 2. The module being reloaded more times than expected

		finalSuccessCount := atomic.LoadInt64(&successCount)
		finalAlreadyProcessingCount := atomic.LoadInt64(&alreadyProcessingCount)
		finalReloadCount := slowModule.getReloadCount()

		t.Logf("Success count: %d, Already processing count: %d, Module reload count: %d",
			finalSuccessCount, finalAlreadyProcessingCount, finalReloadCount)

		// EXPECTED BEHAVIOR: Only one reload should succeed, others should get "already processing"
		// With the race condition, we might see multiple successes
		assert.Equal(t, int64(1), finalSuccessCount, "Only one reload should succeed")
		assert.Equal(t, int64(concurrency-1), finalAlreadyProcessingCount, "Other requests should get 'already processing'")
		assert.Equal(t, int64(1), finalReloadCount, "Module should only be reloaded once")
	})

	t.Run("should_prevent_concurrent_fast_requests", func(t *testing.T) {
		// Test that even very fast reloads don't have race conditions
		// We need to ensure requests arrive simultaneously, not sequentially
		orchestrator := NewReloadOrchestrator()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		// Use a module with a small delay to ensure overlap
		fastModule := &testSlowReloadModule{
			name:        "fast-module",
			reloadDelay: 10 * time.Millisecond, // Small delay to ensure overlap
			reloadCount: 0,
		}

		err := orchestrator.RegisterModule("fast", fastModule)
		require.NoError(t, err)

		// Launch requests simultaneously using a sync barrier
		requests := 20
		var wg sync.WaitGroup
		var startBarrier sync.WaitGroup
		var successCount int64
		var alreadyProcessingCount int64

		startBarrier.Add(1) // Barrier to ensure all goroutines start at once

		for i := 0; i < requests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Wait for all goroutines to be ready
				startBarrier.Wait()

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				err := orchestrator.RequestReload(ctx)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				} else if err.Error() == "reload orchestrator: reload already in progress" {
					atomic.AddInt64(&alreadyProcessingCount, 1)
				}
			}()
		}

		// Release all goroutines at once
		startBarrier.Done()
		wg.Wait()

		finalSuccessCount := atomic.LoadInt64(&successCount)
		finalAlreadyProcessingCount := atomic.LoadInt64(&alreadyProcessingCount)
		finalReloadCount := fastModule.getReloadCount()

		t.Logf("Simultaneous requests - Success: %d, Already processing: %d, Module reloads: %d",
			finalSuccessCount, finalAlreadyProcessingCount, finalReloadCount)

		// With proper race condition fix, only one should succeed
		assert.Equal(t, int64(1), finalSuccessCount, "Only one reload should succeed")
		assert.Equal(t, int64(requests-1), finalAlreadyProcessingCount, "Other requests should get 'already processing'")
		assert.Equal(t, int64(1), finalReloadCount, "Module should only be reloaded once")
	})
}

// Test helper modules for race condition testing

type testSlowReloadModule struct {
	name        string
	reloadDelay time.Duration
	reloadCount int64
	mu          sync.Mutex
}

func (m *testSlowReloadModule) Reload(ctx context.Context, changes []ConfigChange) error {
	// Simulate slow reload operation
	time.Sleep(m.reloadDelay)
	atomic.AddInt64(&m.reloadCount, 1)
	return nil
}

func (m *testSlowReloadModule) CanReload() bool {
	return true
}

func (m *testSlowReloadModule) ReloadTimeout() time.Duration {
	return 30 * time.Second
}

func (m *testSlowReloadModule) getReloadCount() int64 {
	return atomic.LoadInt64(&m.reloadCount)
}

type testFastReloadModule struct {
	name        string
	reloadCount int64
}

func (m *testFastReloadModule) Reload(ctx context.Context, changes []ConfigChange) error {
	// Very fast reload
	atomic.AddInt64(&m.reloadCount, 1)
	return nil
}

func (m *testFastReloadModule) CanReload() bool {
	return true
}

func (m *testFastReloadModule) ReloadTimeout() time.Duration {
	return 30 * time.Second
}

func (m *testFastReloadModule) getReloadCount() int64 {
	return atomic.LoadInt64(&m.reloadCount)
}
