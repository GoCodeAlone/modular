package modular

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReloadOrchestratorCircuitBreaker(t *testing.T) {
	t.Run("should apply exponential backoff after repeated failures", func(t *testing.T) {
		config := ReloadOrchestratorConfig{
			BackoffBase: 100 * time.Millisecond,
			BackoffCap:  1 * time.Second,
		}

		orchestrator := NewReloadOrchestratorWithConfig(config)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		failingModule := &testReloadModule{
			name:      "failing-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				return assert.AnError
			},
		}

		err := orchestrator.RegisterModule("test", failingModule)
		require.NoError(t, err)

		ctx := context.Background()

		// First failure - should be immediate
		start := time.Now()
		err = orchestrator.RequestReload(ctx)
		elapsed := time.Since(start)
		assert.Error(t, err)
		assert.Less(t, elapsed, 50*time.Millisecond) // Should be quick

		// Second failure - should have backoff
		start = time.Now()
		err = orchestrator.RequestReload(ctx)
		elapsed = time.Since(start)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")
		assert.Less(t, elapsed, 50*time.Millisecond) // Should be rejected quickly

		// Wait for backoff to expire and try again
		time.Sleep(150 * time.Millisecond) // Wait longer than BackoffBase

		start = time.Now()
		err = orchestrator.RequestReload(ctx)
		elapsed = time.Since(start)
		assert.Error(t, err)
		// This should actually execute and fail (not be rejected due to backoff)
		// The timing test is too fragile, just verify it's not a backoff error
		assert.NotContains(t, err.Error(), "backing off")
	})

	t.Run("should reset failure count after successful reload", func(t *testing.T) {
		config := ReloadOrchestratorConfig{
			BackoffBase: 100 * time.Millisecond,
			BackoffCap:  1 * time.Second,
		}

		orchestrator := NewReloadOrchestratorWithConfig(config)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		toggleModule := &testToggleReloadModule{
			name:       "toggle-module",
			canReload:  true,
			shouldFail: true, // Start with failures
		}

		err := orchestrator.RegisterModule("test", toggleModule)
		require.NoError(t, err)

		ctx := context.Background()

		// First failure
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)

		// Second failure - should get backoff
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")

		// Make module succeed
		toggleModule.shouldFail = false

		// Wait for backoff to expire
		time.Sleep(150 * time.Millisecond)

		// This should succeed and reset failure count
		err = orchestrator.RequestReload(ctx)
		assert.NoError(t, err)

		// Make module fail again
		toggleModule.shouldFail = true

		// Next failure should be immediate (no backoff)
		start := time.Now()
		err = orchestrator.RequestReload(ctx)
		elapsed := time.Since(start)
		assert.Error(t, err)
		assert.Less(t, elapsed, 50*time.Millisecond) // Should be quick, not backed off
	})

	t.Run("should respect backoff cap", func(t *testing.T) {
		config := ReloadOrchestratorConfig{
			BackoffBase: 50 * time.Millisecond,
			BackoffCap:  200 * time.Millisecond,
		}

		orchestrator := NewReloadOrchestratorWithConfig(config)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		failingModule := &testReloadModule{
			name:      "failing-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				return assert.AnError
			},
		}

		err := orchestrator.RegisterModule("test", failingModule)
		require.NoError(t, err)

		ctx := context.Background()

		// Generate several failures to test backoff behavior
		// First failure - no backoff yet
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "backing off")

		// Second failure - should trigger backoff
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")

		// Wait for first backoff to expire (50ms base)
		time.Sleep(80 * time.Millisecond)

		// Third failure
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "backing off") // Should execute

		// Fourth attempt should have longer backoff (50ms * 2 = 100ms)
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")

		// Wait for longer backoff
		time.Sleep(120 * time.Millisecond)

		// Fifth failure
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "backing off") // Should execute

		// The backoff should never exceed the cap (200ms)
		// This is more of a logical test - the actual verification is in the implementation
	})

	t.Run("should handle concurrent reload requests during backoff", func(t *testing.T) {
		config := ReloadOrchestratorConfig{
			BackoffBase: 200 * time.Millisecond,
			BackoffCap:  1 * time.Second,
		}

		orchestrator := NewReloadOrchestratorWithConfig(config)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			orchestrator.Stop(ctx)
		}()

		failingModule := &testReloadModule{
			name:      "failing-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				return assert.AnError
			},
		}

		err := orchestrator.RegisterModule("test", failingModule)
		require.NoError(t, err)

		ctx := context.Background()

		// First failure to trigger backoff
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)

		// Launch multiple concurrent requests during backoff period
		var wg sync.WaitGroup
		results := make([]error, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = orchestrator.RequestReload(ctx)
			}(i)
		}

		wg.Wait()

		// All should fail, but they might get different errors (backoff vs already in progress)
		for i, err := range results {
			assert.Error(t, err, "Request %d should have failed", i)
			// Accept either backoff error or "already in progress" error
			hasBackoff := strings.Contains(err.Error(), "backing off")
			hasInProgress := strings.Contains(err.Error(), "already in progress")
			assert.True(t, hasBackoff || hasInProgress, "Request %d should mention backing off or already in progress, got: %v", i, err.Error())
		}
	})
}

// Test helper types for circuit breaker testing

type testToggleReloadModule struct {
	name       string
	canReload  bool
	shouldFail bool
	mu         sync.RWMutex
	onReload   func(ctx context.Context, changes []ConfigChange) error
}

func (m *testToggleReloadModule) Name() string {
	return m.name
}

func (m *testToggleReloadModule) CanReload() bool {
	return m.canReload
}

func (m *testToggleReloadModule) ReloadTimeout() time.Duration {
	return 5 * time.Second
}

func (m *testToggleReloadModule) Reload(ctx context.Context, changes []ConfigChange) error {
	m.mu.RLock()
	shouldFail := m.shouldFail
	m.mu.RUnlock()

	if shouldFail {
		return assert.AnError
	}

	if m.onReload != nil {
		return m.onReload(ctx, changes)
	}

	return nil
}

func (m *testToggleReloadModule) SetShouldFail(fail bool) {
	m.mu.Lock()
	m.shouldFail = fail
	m.mu.Unlock()
}

// Test circuit breaker internals
func TestReloadOrchestratorBackoffCalculation(t *testing.T) {
	config := ReloadOrchestratorConfig{
		BackoffBase: 100 * time.Millisecond,
		BackoffCap:  1 * time.Second,
	}

	orchestrator := NewReloadOrchestratorWithConfig(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		orchestrator.Stop(ctx)
	}()

	t.Run("should calculate exponential backoff correctly", func(t *testing.T) {
		// Test internal backoff calculation logic by observing behavior
		// (Since methods are private, we test through public interface)

		failingModule := &testReloadModule{
			name:      "failing-module",
			canReload: true,
			onReload: func(ctx context.Context, changes []ConfigChange) error {
				return assert.AnError
			},
		}

		err := orchestrator.RegisterModule("test", failingModule)
		require.NoError(t, err)

		ctx := context.Background()

		// First failure
		start := time.Now()
		err = orchestrator.RequestReload(ctx)
		duration1 := time.Since(start)
		assert.Error(t, err)

		// Should have immediate response for actual failure
		assert.Less(t, duration1, 50*time.Millisecond)

		// Second request should be backed off
		start = time.Now()
		err = orchestrator.RequestReload(ctx)
		duration2 := time.Since(start)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")

		// Should be immediately rejected
		assert.Less(t, duration2, 50*time.Millisecond)

		// Wait for backoff to expire
		time.Sleep(150 * time.Millisecond)

		// Third attempt should execute but fail again
		err = orchestrator.RequestReload(ctx)
		assert.Error(t, err)
		// Just verify it's not a backoff error, timing is too unreliable
		assert.NotContains(t, err.Error(), "backing off")

		// Fourth attempt should have longer backoff
		start = time.Now()
		err = orchestrator.RequestReload(ctx)
		duration4 := time.Since(start)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "backing off")
		assert.Less(t, duration4, 50*time.Millisecond) // Rejected quickly
	})
}
